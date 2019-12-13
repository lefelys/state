package state

import "reflect"

type valueState struct {
	*group
	key   interface{}
	value interface{}
}

// WithValue returns new State with merged children and value assigned to key.
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
//
// 2. A key can be any type that supports equality and can not be nil.
//
// 3. Packages should define keys as an unexported type to avoid
// collisions.
//
// 4. Packages that define a State key should provide type-safe accessors
// for the values stored using that key (see examples).
func WithValue(key, value interface{}, children ...State) State {
	return withValue(key, value, children...)
}

func withValue(key, value interface{}, children ...State) *valueState {
	if key == nil {
		panic("nil state value key")
	}

	if !reflect.TypeOf(key).Comparable() {
		panic("state value key is not comparable")
	}
	return &valueState{
		group: merge(children...),
		key:   key,
		value: value,
	}
}

// Value returns value assotiated with key from valueState or from its children,
// or nil if it is not found.
func (e *valueState) Value(key interface{}) (value interface{}) {
	if e.key == key {
		return e.value
	}

	return e.group.Value(key)
}

func (e *valueState) DependsOn(children ...State) State {
	return withDependency(e, children...)
}
