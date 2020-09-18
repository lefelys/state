package state

import (
	"context"
	"sync"
)

type dependState struct {
	children *group
	parent   State

	finished chan struct{}
	ready    chan struct{}

	sync.RWMutex
}

// withDependency returns new state with merged parent and children
// with parent's dependency set on children.
func withDependency(parent State, children ...State) *dependState {
	return &dependState{
		children: merge(children...),
		parent:   parent,
		finished: make(chan struct{}),
	}
}

func (d *dependState) Shutdown(ctx context.Context) error {
	return shutdown(ctx, d)
}

func (d *dependState) close() {
	d.children.close()
	<-d.children.finishSig()

	d.parent.close()
	<-d.parent.finishSig()
	d.Done()
}

func (d *dependState) Done() {
	d.Lock()
	defer d.Unlock()
	select {
	case <-d.finished:
		// Already closed
	default:
		close(d.finished)
	}
}

func (d *dependState) Wait() {
	d.children.Wait()
	d.parent.Wait()
}

func (d *dependState) Ready() <-chan struct{} {
	d.Lock()
	defer d.Unlock()

	if d.ready != nil {
		// To avoid memory leaks - ready channel is created only once
		return d.ready
	}

	d.ready = make(chan struct{})

	go func() {
		<-d.children.Ready()
		<-d.parent.Ready()
		close(d.ready)
	}()

	return d.ready
}

func (d *dependState) Err() (err error) {
	if err = d.parent.Err(); err != nil {
		return err
	}

	for _, states := range d.children.states {
		if err = states.Err(); err != nil {
			return err
		}
	}

	return
}

func (d *dependState) Value(key interface{}) (value interface{}) {
	if value = d.parent.Value(key); value != nil {
		return value
	}

	for _, states := range d.children.states {
		if value = states.Value(key); value != nil {
			return value
		}
	}

	return
}

func (d *dependState) DependsOn(children ...State) State {
	return d.dependsOn(children...)
}

func (d *dependState) dependsOn(children ...State) *dependState {
	return withDependency(d, children...)
}

func (d *dependState) finishSig() <-chan struct{} {
	return d.finished
}

func (d *dependState) cause() error {
	err := d.children.cause()
	if err != nil {
		return err
	}

	err = d.parent.cause()
	if err != nil {
		return err
	}

	return nil
}
