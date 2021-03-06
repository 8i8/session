package session

import (
	"time"

	"github.com/8i8/session/ram"
	"github.com/google/uuid"
)

// Sessioner maintains users session data whilst they are logged into
// the application.
type Sessioner interface {
	Set(key string, value interface{}) (err error)
	Get(key string) (value interface{}, err error)
	Del(key string) (err error)
	Valid() (ok bool)
}

// Provider administers concrete sessions, in all but longevity.
type Provider interface {
	Create(sid uuid.UUID, maxage int) (ram.Session, error)
	Restore(sid uuid.UUID) (ram.Session, error)
	Destroy(sid uuid.UUID) error
}

// Timer returns.
type Timer interface {
	Period(t time.Duration) time.Duration
}

// Manager provides an interface for administering sessions, it
// includes a Timer for session timeout.
type Manager interface {
	Provider
	Timer
}

// MemType define the type of memory that the session server is to use.
type MemType int

const (
	// RAM keeps the session store in system ram.
	RAM MemType = iota
)

// manager contains a session provider.
type manager struct {
	Manager
}

// NewManager returns a session manager.
func NewManager(mem MemType) Manager {
	var m manager
	switch mem {
	case RAM:
		m.Manager = ram.Init()
	}
	return m
}

// OptMgrFunc is a function used to set options on the session manager.
type OptMgrFunc func(*manager) OptMgrFunc

// Options is used to set options on a session manager.  Options returns
// a function that contains the data to restore the previous value of
// the last option that it received.
func (m *manager) Options(opts ...OptMgrFunc) (previous OptMgrFunc) {
	for _, opt := range opts {
		previous = opt(m)
	}
	return previous
}

// Period sets the interval between which the stores session timeout
// verification is run. If a sessions timeout value is less than the
// difference between the current time, at periodic intervals, and the
// last used time set in the session, then the session is destroyed.
// The default value is 20 minutes.
func (o OptMgrFunc) Period(t time.Duration) (previous OptMgrFunc) {
	return func(m *manager) OptMgrFunc {
		prev := m.Period(t)
		return o.Period(prev)
	}
}
