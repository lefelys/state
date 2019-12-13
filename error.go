package state

import (
	"sync"
)

type errState struct {
	*group
	err error

	sync.RWMutex
}

// WithError returns new State with merged children and assigned err to it.
func WithError(err error, children ...State) State {
	return withError(err, children...)
}

func withError(err error, children ...State) *errState {
	return &errState{
		group: merge(children...),
		err:   err,
	}
}

// Err returns error assigned to errState
func (e *errState) Err() (err error) {
	e.RLock()
	defer e.RUnlock()

	return e.err
}

func (e *errState) DependsOn(children ...State) State {
	return withDependency(e, children...)
}
