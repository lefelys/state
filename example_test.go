package state

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

func ExampleWithShutdown() {
	st := func() State {
		st, tail := WithShutdown()
		ticker := time.NewTicker(1 * time.Second)

		go func() {
			for {
				select {
				// receive a shutdown signal
				case <-tail.End():
					ticker.Stop()

					// send a signal that the shutdown is complete
					tail.Done()
					return
				case <-ticker.C:
					// some job
				}
			}
		}()

		return st
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := st.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleWithShutdown_dependency() {
	runJob := func(name string) State {
		st, tail := WithShutdown()
		go func() {
			<-tail.End()

			fmt.Println("shutdown " + name)

			tail.Done()
		}()

		return st
	}

	st1 := runJob("job 1")
	st2 := runJob("job 2")
	st3 := runJob("job 3")

	// st3 will be shut down first, then st2, then st1
	st := st1.DependsOn(st2).DependsOn(st3)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := st.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Output: shutdown job 3
	// shutdown job 2
	// shutdown job 1
}

func ExampleWithShutdown_dependencyWrap() {
	st1 := func() State {
		st, tail := WithShutdown()
		go func() {
			<-tail.End()
			fmt.Println("shutdown job 1")
			tail.Done()
		}()

		return st
	}()

	// st1 will be shut down first, then st2
	st2, tail := WithShutdown(st1)

	go func() {
		<-tail.End()
		fmt.Println("shutdown job 2")
		tail.Done()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := st2.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Output: shutdown job 1
	// shutdown job 2
}

func ExampleWithWait() {
	st := func() State {
		st, tail := WithWait()

		for i := 1; i <= 3; i++ {
			tail.Add(1)

			go func(i int) {
				<-time.After(time.Duration(i) * 50 * time.Millisecond)
				fmt.Printf("job %d ended\n", i)

				tail.Done()
			}(i)
		}

		return st
	}()

	// blocks until state's WaitGroup counter is zero
	st.Wait()

	// Output: job 1 ended
	// job 2 ended
	// job 3 ended
}

func ExampleWithError() {
	st := func() State {
		return WithError(errors.New("error"))
	}()

	if err := st.Err(); err != nil {
		fmt.Println(err)
	}

	// Output: error
}

func ExampleWithErrorGroup() {
	st := func() State {
		st, tail := WithErrorGroup()

		go func() {
			tail.Error(errors.New("error"))
		}()

		return st
	}()

	time.Sleep(100 * time.Millisecond)

	if err := st.Err(); err != nil {
		fmt.Println(err)
	}

	// Output: error
}

func ExampleWithAnnotation() {
	st := func() State {
		st, tail := WithErrorGroup()

		go func() {
			tail.Error(errors.New("error"))
		}()

		return WithAnnotation("my job", st)
	}()

	time.Sleep(100 * time.Millisecond)

	if err := st.Err(); err != nil {
		fmt.Println(err)
	}

	// Output: my job: error
}

func ExampleWithAnnotation_shutdown() {
	st := func() State {
		st, tail := WithShutdown()

		go func() {
			<-tail.End()
			<-time.After(1 * time.Second)
			tail.Done()
		}()

		return WithAnnotation("my job", st)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	err := st.Shutdown(ctx)
	if err != nil {
		fmt.Println(err)
	}

	// Output: my job: timeout expired
}

func ExampleMerge() {
	runJob := func(name string, duration time.Duration) State {
		st, tail := WithShutdown()
		go func() {
			<-tail.End()

			<-time.After(duration)
			fmt.Println("shutdown " + name)

			tail.Done()
		}()

		return st
	}

	st1 := runJob("job 1", 50*time.Millisecond)
	st2 := runJob("job 2", 100*time.Millisecond)
	st3 := runJob("job 3", 150*time.Millisecond)

	st := Merge(st1, st2, st3)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := st.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Output: shutdown job 1
	// shutdown job 2
	// shutdown job 3
}

func ExampleWithValue() {
	type key int

	var greetingKey key

	getGreeting := func(st State) (c chan string, ok bool) {
		value := st.Value(greetingKey)
		if value != nil {
			return value.(chan string), true
		}

		return
	}

	st := func() State {
		c := make(chan string)
		st := WithValue(greetingKey, c)

		go func() {
			c <- "hi"
		}()

		return st
	}()

	c, ok := getGreeting(st)
	if ok {
		fmt.Println(<-c)
	}

	// Output: hi
}
