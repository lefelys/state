package main

import (
	"fmt"
	"github.com/lefelys/state"
	"time"
)

func main() {
	st := state.Merge(StartJob1(), StartJob2())

	st.Wait()

}

func StartJob1() state.State {
	st, tail := state.WithWait()

	for i := 0; i < 5; i++ {
		tail.Add(1)
		go func() {
			time.Sleep(1 * time.Second)
			fmt.Println("done job 1")
			tail.Done()
		}()
	}

	return st
}

func StartJob2() state.State {
	st, tail := state.WithWait()

	for i := 0; i < 5; i++ {
		tail.Add(1)
		go func() {
			time.Sleep(2 * time.Second)
			fmt.Println("done job 2")
			tail.Done()
		}()
	}

	return st
}
