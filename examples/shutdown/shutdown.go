package main

import (
	"context"
	"fmt"
	"github.com/lefelys/state"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func StartJob(name string) state.State {
	st, tail := state.WithShutdown()
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		for {
			select {
			case <-tail.End():
				ticker.Stop()
				fmt.Println("shutdown " + name)
				tail.Done()
				return
			case <-ticker.C:
				/*...*/
			}
		}
	}()

	return st
}

func main() {
	st1 := StartJob("job1")
	if err := st1.Err(); err != nil {
		log.Fatal(err)
	}

	st2 := StartJob("job2")
	if err := st2.Err(); err != nil {
		log.Fatal(err)
	}

	// job2 will be shut down first, then job1
	appSt := st1.DependsOn(st2)

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-shutdownSig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := appSt.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if err := appSt.Err(); err != nil {
		log.Fatal(err)
	}
}
