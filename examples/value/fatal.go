package main

import (
	"errors"
	"fmt"
	"github.com/lefelys/state"
	"log"
	"net/http"
)

type Fatality interface {
	Fatal() <-chan error
}

type Fatal struct {
	errCh chan error
}

func (f Fatal) Error(err error) {
	f.errCh <- err
}

func (f Fatal) Errorf(format string, a ...interface{}) {
	f.errCh <- fmt.Errorf(format, a...)
}

func (f Fatal) Fatal() <-chan error {
	return f.errCh
}

type key int

var fatalKey key

// WithFatality returns state with Fatality value set and its tail.
// If Fatal state already present in children - returns merged children
// and its tail
func WithFatality(children ...state.State) (state.State, state.ErrTail) {
	for _, child := range children {
		if f, ok := FatalityFromState(child); ok {
			return state.Merge(children...), f.(state.ErrTail)
		}
	}

	fatal := &Fatal{
		errCh: make(chan error),
	}

	return state.WithValue(fatalKey, fatal, children...), fatal
}

// FatalityFromState returns Fatality value stored in st. If there are
// multiple values associated with fatalKey in the tree, only
// the topmost and the leftmost will be returned.
func FatalityFromState(st state.State) (Fatality, bool) {
	f, ok := st.Value(fatalKey).(Fatality)
	return f, ok
}

type Server struct {
	*http.Server
}

func NewServer() state.State {
	server := &Server{
		Server: &http.Server{
			Addr:    ":8000",
			Handler: http.DefaultServeMux,
		},
	}

	st := server.Start()

	return state.WithAnnotation("http server", st)
}

func (s *Server) Start() state.State {
	st, fatalTail := WithFatality()

	go func() {
		err := s.Server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			fatalTail.Errorf("server fatal error: %w", err)
		}
	}()

	return st
}

func main() {
	serverSt := NewServer()
	if err := serverSt.Err(); err != nil {
		log.Fatal(err)
	}

	fatalSt, ok := FatalityFromState(serverSt)
	if !ok {
		log.Fatal("fatality state not found")
	}

	if err := <-fatalSt.Fatal(); err != nil {
		log.Fatal(err)
	}
}
