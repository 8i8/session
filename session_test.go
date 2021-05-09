package session

import (
	"testing"

	"github.com/google/uuid"
)

func TestDeactivate(t *testing.T) {
	const fname = "TestDeactivate"

	// Create a manager.
	m := NewManager(RAM)

	// Activate a session.
	id := uuid.New()
	sess, err := m.Create(id, 0)
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}
	err = sess.Set("num", 123)
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}

	// Remove the session.
	m.Destroy(id)

	// Create a new session.
	sess2, err := m.Create(id, 0)
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}

	// Should return false but not an error.
	one, err := sess2.Get("num")
	if err != Err09Record {
		t.Errorf("%s: want ErrNotFound got (%T, %+v)",
			fname, err, err)
	}

	// a should not contain an int.
	var a int
	a, ok := one.(int)
	if ok {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, one, one)
	}
	if a != 0 {
		t.Errorf("%s: want (int 0) got (%T %+v)", fname, a, a)
	}
}

func TestActivate(t *testing.T) {
	const fname = "TestNewManager"
	var ok bool
	m := NewManager(RAM)
	id := uuid.New()
	sess, err := m.Create(id, 0)
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}

	err = sess.Set("one", 1)
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}
	err = sess.Set("23", 123)
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}
	sess2, err := m.Create(id, 0)
	if err != Err08Resource {
		t.Errorf("%s: want %q got %q", fname, Err08Resource,
			err)
	}
	sess2, err = m.Restore(id)
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}
	one, err := sess2.Get("one")
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}
	var a int
	a, ok = one.(int)
	if !ok {
		t.Errorf("%s: want 1 got (%T, %+v)", fname, a, a)
	}
	if a != 1 {
		t.Errorf("%s: want 1 got %+v", fname, a)
	}

	n, err := sess2.Get("23")
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}
	a, ok = n.(int)
	if !ok {
		t.Errorf("%s: want 123 got (%T, %+v)", fname, n, n)
	}
	if a != 123 {
		t.Errorf("%s: want 123 got %+v", fname, a)
	}

	sess.Del("one")

	one, err = sess2.Get("one")
	if err != Err09Record {
		t.Errorf("%s: want ErrNotFound got (%T, %+v)", fname, err, err)
	}
	if one != nil {
		t.Errorf("%s: want nil got %+v", fname, a)
	}

	err = sess.Set("23", "hello")
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}

	str, err := sess2.Get("23")
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}
	str1, ok := str.(string)
	if !ok {
		t.Errorf("%s: want \"hello\" got (%T, %+v)", fname, n, n)
	}
	if str1 != "hello" {
		t.Errorf("%s: want \"hello\" got %q", fname, a)
	}
}

type Inter interface {
	Do() string
}

func retInterface(d Inter) Inter {
	return d
}

type doingit struct {
	do string
}

func (d doingit) Do() string {
	return d.do
}

func TestInterface(t *testing.T) {
	const fname = "TestInterface"

	str := "something passed"
	data := retInterface(doingit{do: str})

	m := NewManager(RAM)
	id := uuid.New()
	sess, err := m.Create(id, 0)
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}

	err = sess.Set("data", data)
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}

	ret, err := sess.Get("data")
	if err != nil {
		t.Errorf("%s: want <nil> got (%T, %+v)", fname, err, err)
	}
	r, ok := ret.(Inter)
	if !ok {
		t.Errorf("%s: want ok", fname)
	}

	str2 := r.Do()
	if str2 != str {
		t.Errorf("%s: want %q got %q", fname, str, str2)
	}
}
