package state

import (
	"sync"
)

type waitState struct {
	*group
	sync.WaitGroup
}

// WaitTail detaches after waitable state initialization.
// The tail is supposed to stay in a background job associated with
// created State.
//
// WaitTail uses sync.WaitGroup and shares all its mechanics.
type WaitTail interface {
	// Done calls sync.WaitGroup's Done method
	Done()

	// Add calls sync.WaitGroup's Add method
	Add(i int)
}

// WithWait returns new waitable State with merged children.
//
// The returned WaitTail is used to increment and decrement State's WaitGroup counter.
func WithWait(children ...State) (State, WaitTail) {
	s := withWait(children...)

	return s, s
}

func withWait(children ...State) *waitState {
	return &waitState{
		group: merge(children...),
	}
}

//  Wait blocks until States's and States's children counters are zero.
func (w *waitState) Wait() {
	w.WaitGroup.Wait()
	w.group.Wait()
}

func (w *waitState) DependsOn(children ...State) State {
	return withDependency(w, children...)
}
