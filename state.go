// The state package provides simple state management primitives for go applications.
//
// The package defines the State type, which carries errors, wait groups,
// shutdown signals and other values from application's background jobs.
//
// The State type is aggregative - it contains multiple states in tree form,
// allowing setting dependencies for graceful shutdown between them and merging
// multiple independent states.
//
// To aggregate the application's states, functions that initialize background
// jobs create suitable State and propagate it up in the calls stack to the
// layer where it will be handled, optionally merging it with other states,
// setting dependencies between them and annotating along the way.
//
// Programs that use State should follow these rules to keep interfaces
// consistent:
//
// 1. All functions that initialize application-scoped background jobs should
// return State as its last return value.
//
// There might be special cases, when returning state as the last return value is
// not possible, for example - when using dependency injection packages. To handle
// this case, embed State into dependency's return value:
//
//  type Server struct {
//  	state.State
//
//  	server *http.Server
//  }
//
//  type Updater interface {
//  	state.State
//
//  	Update() error
//  }
//
//  func NewApp(server *Server, updater Updater) state.State {
//  	 st := server.DependsOn(updater)
//
//  	 /*...*/
//  }
//
// 2. If an error can occur during initialization it is still should be returned
// as State using function WithError.
//
// 3. Never return nil State - return Empty() instead, or do not return State
// at all if it is not needed.
//
// 4. Every background job should be shutdownable and/or waitable.
package state

import (
	"context"
	"errors"
)

// State carries errors, wait groups, shutdown signals and other values
// from application's background jobs in tree form.
//
// State is not reusable.
//
// State's methods may be called by multiple goroutines simultaneously.
type State interface {
	// Err returns the first encountered error in this state.
	// While error is propagated from bottom to top, it is being annotated
	// by annotation states in a chain. Annotation uses introduced in
	// go 1.13 errors wrapping.
	//
	// Successive calls to Err may not return the same value, but it will
	// never return nil after the first error occurred.
	Err() error

	// Wait blocks until all counters of WaitGroups in this state are zero.
	// It uses sync.Waitgroup under the hood and shares all its mechanics.
	Wait()

	// Shutdown gracefully shuts down this state.
	// Ths shutdown occurs from bottom to top: parents shut down their
	// children, wait until all of them are successfully shut down and
	// then shut down themselves.
	//
	// If ctx expires before the shutdown is complete, Shutdown tries
	// to find the first full path of unclosed children to accumulate
	// annotations and returns ErrTimeout wrapped in them.
	// There is a chance that the shutdown will complete during that check -
	// in this case, it is considered as fully completed and returns nil.
	Shutdown(ctx context.Context) error

	// Ready returns a channel that signals that all states in tree are
	// ready. If there is no readiness states in the tree - state is considered
	// as ready by default.
	//
	// If some readiness state in the tree didn't send Ok signal -
	// returned channel blocks forever. It is caller's responsibility to
	// handle possible block.
	Ready() <-chan struct{}

	// Value returns the first found value in this state for key,
	// or nil if no value is associated with key. The tree is searched
	// from top to bottom and from left to right.
	//
	// It is possible to have multiple values associated with the same key,
	// but Value call will always return the topmost and the leftmost.
	//
	// Use state values only for data that represents custom states, not
	// for returning optional values from functions.
	//
	// Other rules for working with Value is the same as in the standard
	// package context:
	//
	// 1. Functions that wish to store values in State typically allocate
	// a key in a global variable then use that key as the argument to
	// state.WithValue and State.Value.
	// 2. A key can be any type that supports equality and can not be nil.
	// 3. Packages should define keys as an unexported type to avoid
	// collisions.
	// 4. Packages that define a State key should provide type-safe accessors
	// for the values stored using that key (see examples).
	Value(key interface{}) (value interface{})

	// DependsOn creates a new state from the original and children.
	// The new state ensures that during shutdown it will shut down children
	// first, wait until all of them are successfully shut down and then shut
	// down the original state.
	DependsOn(children ...State) State

	// closer is a private inteface used for graceful shutdown. It is
	// necessary to have it in exported interface for cases of embedding
	// State into another struct.
	closer
}

var (
	// ErrTimeout is the error returned by State.Shudown when shutdown's
	// timeout is expired
	ErrTimeout = errors.New("timeout expired")

	// closedchan is a reusable closed channel.
	closedchan = make(chan struct{})
)

func init() {
	close(closedchan)
}
