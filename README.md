# state

The package provides a simple mechanism for managing application's background jobs, allowing easy to manage **graceful shutdown**, **waiting** for completion, **error propagation** and **custom states** creation.


## Features
- **Graceful Shutdown:** easily create graceful shutdown with dependencies for your application without juggling with channels and goroutines
- **Waiting:** wait for completion for all waitable background jobs
- **Annotating**: quickly find the cause of errors or frozen shutdowns
- **Error Group**: errors propagation across the tree of states
- **Custom states**

## Prerequisites

Package state defines the `State` type, which carries errors, wait groups, shutdown signals and other values from application's background jobs.

The `State` type is aggregative - it contains multiple states in tree form, allowing setting dependencies for graceful shutdown between them and merging multiple independent states.

To aggregate the application's states, functions that initialize background jobs create suitable `State` and propagate it up in the calls stack to the layer where it will be handled, optionally merging it with other states, setting dependencies between them and annotating along the way.

State has some mechanic similarities with context package.

In the case of context, the `Context` type carries **request-scoped** deadlines, cancelation signals, and other values across API boundaries and between processes.

A common example of `Context` usage – application receives a user request, creates context and makes a request to external API with it:
```
1. o-------------------------------------->  user request
2.          o------------------->            API request
```

By propagating cancelable `Context`, calling the `CancelFunc` during user request cancels the parent and all its children simultaneously:

```
1. o------------[cancel]-x                   user request
2.          o------------x                   API request
```

`Context` is short-lived and is not supposed to be stored or reused.

`State`, on the other hand, is **application-scoped**. It provides a type similar to `Context` for controlling long-living background jobs. Unlike Context, `State` propagates bottom-top during the app initialization phase and it is ok to store it, but not reuse.

Simple example – application initializes consumer and processor to work in background:
```
1. o-------------------------------------->  consumer
2.          o----------------------------->  processor
```
If we want to shut down the application, `Context` canceling semantics is not applicable - a simultaneous shutdown of consumer and processor can cause a data loss. State package will handle this case gracefully:  it will shut down consumer, wait until it signals Ok, and shut down processor after:

```
1. o-----------[close]~~~~[ok]-x             consumer
2.          o-------------[close]~~~~[ok]-x  processor
```

## Getting started

### Requirements

Go 1.13+

### Installing

```
go get -u github.com/lefelys/state
```

### Usage

#### Creation
Create a new `State` using one of the package functions: `WithShutdown`, `WithWait`, `WithErr`, `WithErrorGroup` or `WithValue`:

```go
st := state.WithErr(errors.New("error"))
```

#### Tails

Functions `WithShutdown`, `WithWait`, and `WithErrorGroup` in addition to a new `State` returns detached tail, which must be used for signaling in a background job associated with the `State`. In the case of `WithShutdown`, it detaches `ShutdownTail` interface with two methods:
- `End() <-chan struct{}` - returns a channel that's closed when work done on behalf of tail's `State` should be shut down.
- `Done()` - sends a signal that the shutdown is complete.

```go
st, tail := state.WithShutdown()

go func() {
	for {
		select {
		case <-tail.End():
			fmt.Println("shutdown job")
			tail.Done()
			return
		default:
			/*...*/
		}
	}
}()
```

#### Propagation
Created `State` should be propagated up in the calls stack, where it will be handled:

```go
func StartJob() state.State {
	st, tail := state.WithShutdown()

	go func() {
		/*...*/
	}()

	return st
}

func main() {
	jobSt := StartJob()

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-shutdownSig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := jobSt.Shutdown(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
```


#### Merging and annotation
To merge multiple States use function `Merge`. States can also be merged using function `WithAnnotation` - it will help to find the cause of errors or frozen shutdowns:

```go
func StartJobs() state.State {
	st1 := startJob1()
	st2 := startJob2()

	return state.WithAnnotation("job1 and job2", st1, st2)
}

func main() {
	st := StartJobs()

	/*...*/

	err := st.Shutdown(ctx)
	if err != nil {
		log.Fatal(err) // "job1 and job2: timeout expired"
	}
}
```

#### Dependency

Dependency is used for graceful shutdown: dependent State will shut down its children first, wait until all of them are successfully shut down and then shut down itself.
There are 2 ways to create a dependency between States:

1) By passing children States to State initializer:

```go
st1 := startJob1()
st2 := startJob2()

// st3 depends on st1 and st2
st3, tail := state.WithShutdown(st1, st2)
```
2) By calling an existing State's `DependsOn` method, which returns a new `State` with set dependencies:

```go
st1 := StartJob1()
st2 := startJob2()
st3 := startJob3()

// st is the merged st1, st2 and st3 with st3 dependency set on st1 and st2
st := st3.DependsOn(st1, st2)
```
### Recommendations

Programs that use State should follow these rules to keep interfaces consistent:
1) All functions that initialize application-scoped background jobs should return `State` as its **last return value**.
2) If an error can occur during initialization it is still should be returned as `State` using function `WithError`.
3) Never return nil `State` - return `Empty()` instead, or do not return `State` at all if it is not needed.
4) Every background job should be shutdownable and/or waitable.

There might be special cases, when returning state as the last return value is not possible, for example - when using dependency injection packages. To handle this case, embed `State` into dependency's return value:

```go
package app

import "github.com/lefelys/state"

type Server struct {
	state.State

	server *http.Server
}

type Updater interface {
	state.State

	Update() error
}

func NewApp(server *Server, updater Updater) state.State {
	 st := server.DependsOn(updater)

	 /*...*/
}
```


## Examples

See [examples](examples/)
