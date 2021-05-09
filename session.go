package session

import (
	"time"

	"github.com/8i8/http/session/ram"
	"github.com/google/uuid"
)

// Provider is an interface for the session manager.
type Provider interface {
	Create(sid uuid.UUID, maxage int) (Session, error)
	Restore(sid uuid.UUID) (Session, error)
	Destroy(sid uuid.UUID) error
}

// TimeOut provides an encapulated addition to the Provider interface,
// such that it be possible to set the session providers timeout period.
type TimeOut interface {
	SetPeriod(t time.Duration) time.Duration
}

// Manager is the interface into which the provider is held within
// the Manager struct.
type Manager interface {
	Provider
	TimeOut
}

// MemType define the type of memory that the session server is to use.
type MemType int

const (
	// RAM keeps the session store in system ram.
	RAM MemType = iota
)

// manager contains a session provider.
type manager struct {
	storeManager
}

// NewManager returns a session manager.
func NewManager(mem MemType) storeManager {
	var m manager
	switch mem {
	case RAM:
		m.storeManager = ram.Init()
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
		prev := m.SetPeriod(t)
		return o.Period(prev)
	}
}
