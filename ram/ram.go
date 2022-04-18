package ram

import (
	"errors"
	"fmt"
	"time"

	"github.com/8i8/log"
	"github.com/google/uuid"
)

const pkg = "session"
var ErrNoSession = errors.New("session does not exist")
var ErrPoorForm = errors.New("poorly formed uuid")
var ErrTimedOut = errors.New("session timed out")
var ErrNoData = errors.New("data not found in session")

// valueStore is the providrs data storage.
type valueStore map[interface{}]interface{}

// The timeout period within a session is calulated using the value of
// 'period' set within the store as dividen, devided by this divisors
// value when the session is created.
var divisor = time.Duration(2)

// defaultPeriod is the default period, in minutes, of the running of
// the sessions cleanup function.
var defaultPeriod = 20

// cmd is the basic token used to direct the session servers operation.
type cmd int

const (
	create cmd = iota
	activate
	deactivate
	touch
	timecheck
	exit
)

// command is the the type used to drive the session server.
type command struct {
	cmd
	key     uuid.UUID
	maxage  time.Duration
	result  chan Session
	seStore *Store
}

// sessionServer responds to requests for sessions either serving or
// removing them, sessions may be removed either by request or when they
// timeout through lack of activity.
func sessionServer(commands chan command) {
	for c := range commands {
		switch c.cmd {
		case create:
			c.result <- c.create()
		case activate:
			c.result <- c.retrieve()
		case deactivate:
			c.destroy()
			c.result <- Session{}
		case touch:
			c.result <- c.touch()
		case timecheck:
			c.timeout()
			c.result <- Session{}
		default:
			c.def()
			c.result <- Session{}
		}
	}
}

// create makes a session for the given sid, returning and error if the
// session already exists, returning and empty session struct if it does
// not.
func (c command) create() (s Session) {
	const fname = "create"
	_, exists := c.seStore.sessions[c.key]
	if exists {
		if log.Is(log.DEBUG) {
			const event = "Session already in use"
			log.Debug(nil, pkg, fname, event,
				"SID", c.key)
		}
		return Session{}
	}
	s = Session{
		id:       c.key,
		data:     make(valueStore),
		created:  time.Now(),
		modified: time.Now(),
		index:    c.seStore.index,
		sto:      c.seStore,
		maxage:   c.maxage,
		active:   true,
	}
	// If the maxage is not sane, set to half the stores timeout
	// period.
	if c.maxage <= 0 {
		s.maxage = c.seStore.period / divisor
	}
	c.seStore.sessions[c.key] = s
	// Add SID to array and augment index tally.
	c.seStore.array = append(c.seStore.array, c.key)
	c.seStore.index++
	if log.Is(log.DEBUG) {
		const event = "Session created"
		log.Debug(nil, pkg, fname, event, "SID", c.key)
	}
	return s
}

// retrieve returns an active session if one exists for the given sid,
// returning and empty session struct if it does not.
func (c command) retrieve() (s Session) {
	const fname = "activate"
	s, ok := c.seStore.sessions[c.key]
	if ok {
		if log.Is(log.DEBUG) {
			const event = "Session restored"
			log.Debug(nil, pkg, fname, event,
				"SID", c.key)
		}
		// Reset maxage, it may have changed.
		s.maxage = c.maxage
		return s
	}
	if log.Is(log.DEBUG) {
		const event = "Session not found"
		log.Debug(nil, pkg, fname, event, "SID", c.key)
	}
	return Session{}
}

// destroy destroys the session corresponding to the given sid,
// logging an error if there is no session to match the key.
func (c command) destroy() {
	const fname = "cmd.destroy"
	// If the session uuid is valid destroy the session.
	if _, ok := c.seStore.sessions[c.key]; ok {
		c.seStore.destroy(c.key, fname)
		return
	}
	if log.Is(log.DEBUG) {
		const event = "no session to destroy"
		log.Debug(nil, pkg, fname, event, "SID", c.key)
	}
}

// touch updates the modified time of a session, required as sessions
// are being passed by value, not by reference.
func (c command) touch() (s Session) {
	const fname = "cmd.touch"
	// If there is a session update its time.
	s, ok := c.seStore.sessions[c.key]
	if ok {
		s.modified = time.Now()
		c.seStore.sessions[c.key] = s
		return s
	}
	if log.Is(log.DEBUG) {
		const event = "no session for this key"
		log.Debug(nil, pkg, fname, event, "SID", c.key)
	}
	return Session{}
}

// timeout iterates over all of the sessions in the index array,
// destroying any that have a timeout setting that is less than the
// difference between now and the last modified time.
func (c command) timeout() {
	const fname = "cmd.timeout"
	if log.Is(log.DEBUG) {
		const event = "clearing session store"
		log.Debug(nil, pkg, fname, event)
	}
	for key := range c.seStore.sessions {
		s := c.seStore.sessions[key]
		if time.Since(s.modified) > s.maxage {
			c.seStore.destroy(key, fname)
		}
	}
}

// def is the default action when the given command is not recognised.
func (c command) def() {
	const fname = "cmd.def"
	const event = "default fall through"
	log.Fatal(pkg, fname, event, "cmd", c.cmd)
}

// destroy removes the session corresponding to the given SID from the
// store, if it exists, this function is not to be used concurrently and
// has be designed to run only for the dataServer function.
func (s *Store) destroy(key uuid.UUID, sender string) {
	const fname = "cmd.destroy"

	// Retrieve the session.
	se, ok := s.sessions[key]
	if !ok {
		if log.Is(log.ERROR) {
			const event = "no session found"
			log.Err(nil, pkg, fname, event, "SID", key,
				"caller", sender)
		}
		return
	}

	// Remove the SID from the array and diminish the index.
	s.array = append(s.array[:se.index], s.array[se.index+1:]...)
	s.index--

	// Correct the index of all moved sid's.
	for _, uuid := range s.array[se.index:] {
		se := s.sessions[uuid]
		se.index--
		s.sessions[uuid] = se
	}

	// Remove the session from the map.
	delete(s.sessions, key)
	if log.Is(log.DEBUG) {
		const event = "session destroyed"
		log.Debug(nil, pkg, fname, event, "SID", key,
			"caller", sender)
	}
}

// Store contains the session map and array of indices used to track
// sessions.
type Store struct {
	sessions map[uuid.UUID]Session
	array    []uuid.UUID
	index    int
	period   time.Duration
	commands chan command
}

// Init initialises a new ram store.
func Init() *Store {
	var cmds = make(chan command)
	go sessionServer(cmds)
	s := Store{
		sessions: make(map[uuid.UUID]Session),
		period:   time.Minute * time.Duration(defaultPeriod),
		commands: cmds,
	}
	s.startTimer()
	return &s
}

// Create makes a session for which the given SID is the key, returning
// and error if the SID is already in use.
func (s *Store) Create(sid uuid.UUID, maxage int) (se Session, err error) {
	const fname = "Store.Create"
	fail := func(err error) (Session, error) {
		return se, fmt.Errorf("%s: %w", fname, err)
	}
	if sid.Variant() == uuid.Invalid {
		return fail(ErrPoorForm)
	}
	res := make(chan Session)
	c := command{
		cmd:     create,
		key:     sid,
		maxage:  time.Duration(maxage) * time.Second,
		result:  res,
		seStore: s,
	}
	s.commands <- c
	sess := <-res
	if !sess.active {
		return fail(ErrNoSession)
	}
	se = sess
	return
}

// Restore returns a session for which the given SID is the key if it
// exists, returning an error if it does not.
func (s *Store) Restore(sid uuid.UUID) (se Session, err error) {
	const fname = "Store.Restore"
	fail := func(err error) (Session, error) {
		return se, fmt.Errorf("%s: %w", fname, err)
	}

	if sid.Variant() == uuid.Invalid {
		return fail(ErrPoorForm)
	}
	res := make(chan Session)
	c := command{
		cmd:     touch,
		key:     sid,
		result:  res,
		seStore: s,
	}
	s.commands <- c
	sess := <-res
	if !sess.active {
		return fail(ErrNoSession)
	}
	se = sess
	return
}

// Destroy removes a session from the store.
func (s *Store) Destroy(sid uuid.UUID) (err error) {
	const fname = "Store.Destroy"
	if sid.Variant() == uuid.Invalid {
		return fmt.Errorf("%s: %w", fname, ErrNoSession)
	}
	res := make(chan Session)
	c := command{
		cmd:     deactivate,
		key:     sid,
		result:  res,
		seStore: s,
	}
	s.commands <- c
	<-res
	return
}

// Period sets the periodicity for the stores timeout function timer.
func (s *Store) Period(t time.Duration) (previous time.Duration) {
	previous = s.period
	s.period = t
	return
}

// startTimer starts a go routine that periodically clears unused
// sessions from the session store.
func (s *Store) startTimer() {
	res := make(chan Session)
	c := command{
		cmd:     timecheck,
		result:  res,
		seStore: s,
	}
	go func() {
		for {
			time.Sleep(s.period)
			s.commands <- c
			<-res
		}
	}()
}

// touch updates the sessions lastUsed time to now.
func (s *Store) touch(sid uuid.UUID) (se Session) {
	res := make(chan Session)
	c := command{
		cmd:     touch,
		key:     sid,
		result:  res,
		seStore: s,
	}
	s.commands <- c
	se = <-res
	return
}

// Session is a key value pair data store.
type Session struct {
	// Contains non exported fields.
	id       uuid.UUID
	data     valueStore
	created  time.Time
	modified time.Time
	index    int
	sto      *Store
	maxage   time.Duration
	active   bool
}

// Set stores the given key pair value.
func (s Session) Set(key string, value interface{}) (err error) {
	const fname = "Session.Set"
	fail := func(err error) error {
		return fmt.Errorf("%s: %w", fname, err)
	}
	if s.sto == nil || !s.active {
		return fail(ErrTimedOut)
	}
	s = s.sto.touch(s.id)
	if !s.active {
		if log.Is(log.DEBUG) {
			const event = "failed"
			log.Debug(nil, pkg, fname, event,
				"SID", s.id)
		}
		return fail(ErrTimedOut)
	}
	if log.Is(log.DEBUG) {
		const event = "success"
		log.Debug(nil, pkg, fname, event,
			"SID", s.id)
	}
	s.data[key] = value
	return
}

// Get retrieves the value paired with key.
func (s Session) Get(key string) (value interface{}, err error) {
	const fname = "Session.Get"
	fail := func(err error) (interface{}, error) {
		return nil, fmt.Errorf("%s: %w", fname, err)
	}
	if s.sto == nil || !s.active {
		return fail(ErrPoorForm)
	}
	s = s.sto.touch(s.id)
	if !s.active {
		return fail(ErrNoSession)
	}
	value, ok := s.data[key]
	if !ok {
		if log.Is(log.DEBUG) {
			const event = "failed"
			log.Debug(nil, pkg, fname, event,
				"SID", s.id)
		}
		return fail(ErrNoData)
	}
	if log.Is(log.DEBUG) {
		const event = "success"
		log.Debug(nil, pkg, fname, event,
			"SID", s.id)
	}
	return
}

// Del deletes the value paired with key.
func (s Session) Del(key string) (err error) {
	const fname = "Session.Del"
	fail := func(err error) error {
		return fmt.Errorf("%s: %w", fname, err)
	}
	if s.sto == nil || !s.active {
		return fail(ErrPoorForm)
	}
	s = s.sto.touch(s.id)
	if !s.active {
		if log.Is(log.DEBUG) {
			const event = "failed"
			log.Debug(nil, pkg, fname, event,
				"SID", s.id)
		}
		return fail(ErrNoSession)
	}
	delete(s.data, key)
	if log.Is(log.DEBUG) {
		const event = "success"
		log.Debug(nil, pkg, fname, event,
			"SID", s.id)
	}
	return
}

// Valid returns the session active state.
func (s Session) Valid() (ok bool) {
	return s.active
}
