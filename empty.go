package state

import "context"

type emptyState struct{}

// Empty returns new empty State
func Empty() State                                     { return emptyState{} }
func (e emptyState) Err() error                        { return nil }
func (e emptyState) Shutdown(_ context.Context) error  { return nil }
func (e emptyState) Wait()                             {}
func (e emptyState) Value(_ interface{}) interface{}   { return nil }
func (e emptyState) DependsOn(children ...State) State { return withDependency(e, children...) }
func (e emptyState) close()                            {}
func (e emptyState) finishSig() <-chan struct{}        { return closedchan }
func (e emptyState) cause() error                      { return nil }
