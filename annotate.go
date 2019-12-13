package state

import (
	"context"
	"fmt"
)

type annotationState struct {
	*group

	annotation string
}

// WithAnnotation returns new state with merged children and assigned annotation to it.
func WithAnnotation(message string, children ...State) State {
	return withAnnotation(message, children...)
}

func withAnnotation(message string, children ...State) *annotationState {
	return &annotationState{
		group:      merge(children...),
		annotation: message,
	}
}

// Err returns the first encountered error in State's children annotated
// with state's annotation.
// Returns nil if no errors found.
func (a *annotationState) Err() error {
	for _, m := range a.states {
		if err := m.Err(); err != nil {
			return fmt.Errorf("%s: %w", a.annotation, err)
		}
	}

	return nil
}

// Shutdown shuts down state's children and returns annotated shutdown error.
// Returns nil no errors occurred.
func (a *annotationState) Shutdown(ctx context.Context) error {
	if err := a.group.Shutdown(ctx); err != nil {
		return fmt.Errorf("%s: %w", a.annotation, err)
	}

	return nil
}

func (a *annotationState) DependsOn(children ...State) State {
	return withDependency(a, children...)
}

func (a *annotationState) cause() error {
	if err := a.group.cause(); err != nil {
		return fmt.Errorf("%s: %w", a.annotation, err)
	}

	return nil
}
