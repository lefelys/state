package state

import "fmt"

// ErrTail detaches after error group state initialization.
// The tail is supposed to stay in a background job associated with
// created state and used to assign error to the state.
type ErrTail interface {
	// Error assigns err to associated state.
	// If the state already has an error - does nothing.
	Error(err error)

	// Errorf formats according to a format specifier and assigns
	// the string to associated state as a value that satisfies error.
	// If the state already has an error - does nothing.
	Errorf(format string, a ...interface{})
}

type errGroupState struct {
	*errState
}

// WithErrorGroup returns new state with merged children that can
// store an error.
//
// The returned ErrTail is used to assign error to the state.
func WithErrorGroup(children ...State) (State, ErrTail) {
	b := withErrorGroup(children...)
	return b, b
}

func withErrorGroup(children ...State) *errGroupState {
	return &errGroupState{errState: withError(nil, children...)}
}

// Error assigns err to the state.
//
// If the state already has an error - does nothing.
func (e *errGroupState) Error(err error) {
	if err != nil {
		e.Lock()
		if e.err == nil {
			e.err = err
		}
		e.Unlock()
	}
}

// Errorf formats according to a format specifier and assigns
// the string to the state as a value that satisfies error.
//
// If the state already has an error - does nothing.
//
// Uses fmt.Errorf thus supports error wrapping with %w verb.
func (e *errGroupState) Errorf(format string, a ...interface{}) {
	e.Error(fmt.Errorf(format, a...))
}

func (e *errGroupState) DependsOn(children ...State) State {
	return withDependency(e, children...)
}
