package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/lefelys/state"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Generator struct{}

func NewGenerator() (out chan time.Time, st state.State) {
	c := Generator{}
	out, st = c.Start()

	return out, state.WithAnnotation("generator", st)
}

func (c *Generator) Start() (chan time.Time, state.State) {
	st, tail := state.WithShutdown()
	ticker := time.NewTicker(1 * time.Second)
	out := make(chan time.Time)

	go func() {
		for {
			select {
			case <-tail.End():
				ticker.Stop()
				fmt.Println("generator shutdown")
				tail.Done()
				return
			case out <- <-ticker.C:
			}
		}
	}()

	return out, st
}

type Processor struct{}

func NewProcessor(in <-chan time.Time) state.State {
	p := Processor{}
	st := p.Start(in)

	return state.WithAnnotation("processor", st)
}

func (p *Processor) Start(in <-chan time.Time) (st state.State) {
	st, tail := state.WithShutdown()

	go func() {
		for {
			select {
			case <-tail.End():
				fmt.Println("processor shutdown")
				tail.Done()
				return
			case t := <-in:
				fmt.Println(t)
			}
		}
	}()

	return
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

type key int

var fatalKey key

func getServerFatalCh(st state.State) chan error {
	value := st.Value(fatalKey)
	if value != nil {
		return value.(chan error)
	}

	return nil
}

func (s *Server) Start() state.State {
	shutdownSt, shutdownTail := state.WithShutdown()
	errSt, errTail := state.WithErrorGroup()
	fatal := make(chan error)
	serverFatalSt := state.WithValue(fatalKey, fatal)

	go func() {
		err := s.Server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			fatal <- err
		}
	}()

	go func() {
		<-shutdownTail.End()

		// context.Background() never expires, so Server's Shutdown call may
		// only return errors from closing Server's underlying Listener(s).
		err := s.Server.Shutdown(context.Background())
		if err != nil {
			errTail.Errorf("shutdown error from server: %w", err)
		}
		fmt.Println("server shutdown")
		shutdownTail.Done()
	}()

	return state.Merge(shutdownSt, errSt, serverFatalSt)
}

func main() {
	out, generatorSt := NewGenerator()
	if err := generatorSt.Err(); err != nil {
		log.Fatal(err)
	}

	processorSt := NewProcessor(out)
	if err := processorSt.Err(); err != nil {
		log.Fatal(err)
	}

	serverSt := NewServer()
	if err := serverSt.Err(); err != nil {
		log.Fatal(err)
	}

	// generator will be shut down first, then processor, then server
	appState := serverSt.
		DependsOn(processorSt).
		DependsOn(generatorSt)

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case err := <-getServerFatalCh(appState):
		log.Fatalf("fatal error: %v", err)
	case <-shutdownSig:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := appState.Shutdown(ctx)
		if err != nil {
			log.Fatal(err)
		}

		if err := appState.Err(); err != nil {
			log.Fatal(err)
		}
	}
}
