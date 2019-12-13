package state

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestParallel(t *testing.T) {
	t.Run("state", func(t *testing.T) {
		// Group
		t.Run("GroupClose", GroupCloseTest)
		t.Run("GroupSuccessiveClose", GroupSuccessiveCloseTest)
		t.Run("GroupError", GroupErrorTest)
		t.Run("GroupNilChild", GroupNilChildTest)

		// Shutdown
		t.Run("ShutdownWrap", ShutdownWrapTest)
		t.Run("ShutdownSuccessiveDone", ShutdownSuccessiveDoneTest)
		t.Run("ShutdownSuccessiveCall", ShutdownSuccessiveCallTest)
		t.Run("ShutdownTimeout", ShutdownTimeoutTest)
		t.Run("ShutdownUnclosed", ShutdownUnclosedTest)

		// Wait
		t.Run("Wait", WaitTest)

		// Value
		t.Run("ValueWrap", ValueWrapTest)
		t.Run("ValueChildren", ValueChildrenTest)
		t.Run("ValueNilPanic", ValueNilPanicTest)
		t.Run("ValueComparablePanic", ValueComparablePanicTest)

		// Annotation
		t.Run("AnnotationError", AnnotationErrorTest)
		t.Run("AnnotationShutdownTimeout", AnnotationShutdownTimeoutTest)
		t.Run("AnnotationChildShutdownTimeout", AnnotationChildShutdownTimeoutTest)
		t.Run("AnnotationNilError", AnnotationNilErrorTest)
		t.Run("AnnotationNilShutdownError", AnnotationNilShutdownErrorTest)
		t.Run("AnnotationUnclosed", AnnotationUnclosedTest)

		// Error
		t.Run("Error", ErrorTest)

		// Error group
		t.Run("ErrorGroup", ErrorGroupTest)
		t.Run("ErrorGroupErrorf", ErrorGroupErrorfTest)

		// Empty
		t.Run("Empty", EmptyTest)

		// Dependency
		t.Run("DependencyShutdown", DependencyShutdownTest)
		t.Run("DependencyShutdownChain", DependencyShutdownChainTest)
		t.Run("DependencyShutdownSuccessiveClose", DependencyShutdownSuccessiveCloseTest)
		t.Run("DependencyShutdownChildrenTimeout", DependencyShutdownChildrenTimeoutTest)
		t.Run("DependencyShutdownParentTimeout", DependencyShutdownParentTimeoutTest)
		t.Run("DependencyShutdownUnclosed", DependencyShutdownUnclosedTest)
		t.Run("DependencyWait", DependencyWaitTest)
		t.Run("DependencyErrorParent", DependencyErrorParentTest)
		t.Run("DependencyErrorChildren", DependencyErrorChildrenTest)
		t.Run("DependencyErrorNil", DependencyErrorNilTest)
		t.Run("DependencyValueParent", DependencyValueParentTest)
		t.Run("DependencyValueChildren", DependencyValueChildrenTest)
		t.Run("DependencyAnnotation", DependencyAnnotationTest)
	})
}

const (
	failTimeout = 100 * time.Millisecond

	errInitClosed    = "shutdown state initialized closed"
	errNotClosed     = "shutdown state didn't trigger close"
	errNotFinished   = "shutdown state didn't finish"
	errClosed        = "unexpected close of shutdown state"
	errFinished      = "unexpected finish of shutdown state"
	errTimeout       = "unexpected timeout of shutdown state"
	errNotWaited     = "wait state didn't wait"
	errFinishWaiting = "wait state didn't finish waiting"
)

func runShutdownable(tail ShutdownTail) (okDone chan struct{}) {
	okDone = make(chan struct{})

	go func() {
		<-tail.End()
		<-okDone
		tail.Done()
	}()

	return
}

func runWaitable(tail WaitTail) (okWait chan struct{}) {
	okWait = make(chan struct{})
	tail.Add(1)

	go func() {
		<-okWait
		tail.Done()
	}()

	return
}

func isDone(cc ...chan struct{}) bool {
	for _, c := range cc {
		select {
		case <-c:
		default:
			return false
		}
	}
	return true
}

func isNotDone(cc ...<-chan struct{}) bool {
	for _, c := range cc {
		select {
		case <-c:
			return false
		default:
		}
	}
	return true
}

func closeChanAndPropagate(cc ...chan struct{}) {
	for _, c := range cc {
		close(c)
	}
	time.Sleep(failTimeout)
}

// Group

func GroupCloseTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withShutdown()
		st2 = withShutdown()

		okDone1 = runShutdownable(st1)
		okDone2 = runShutdownable(st2)

		st3 = merge(st1, st2)
	)

	go st3.close()
	closeChanAndPropagate(okDone1, okDone2)

	switch {
	case isNotDone(st1.end, st2.end, st3.done):
		t.Error(errNotClosed)
	case isNotDone(st1.done, st2.done, st3.finished):
		t.Error(errNotFinished)
	}
}

func GroupSuccessiveCloseTest(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("successive close call caused panic")
		}
	}()

	var (
		st1 = withShutdown()

		okDone1 = runShutdownable(st1)

		st3 = merge(st1)
	)

	closeChanAndPropagate(okDone1)
	st3.close()
	st3.close()
}

func GroupErrorTest(t *testing.T) {
	t.Parallel()
	var (
		err = errors.New("test")
		st1 = withError(err)
		st2 = emptyState{}
		st3 = emptyState{}

		st4 = merge(st1, st2, st3)
		st5 = merge(st2, st3)
	)

	if err := st4.Err(); err == nil {
		t.Errorf("group state with error state didn't return error")
	}

	if err := st5.Err(); err != nil {
		t.Errorf("group state without error state returned error")
	}
}

func GroupNilChildTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = emptyState{}
		st2 = merge(st1, nil)
	)

	if len(st2.states) != 1 {
		t.Errorf("wrong number of group children: want 1, have %d", len(st2.states))
		return
	}

	if st2.states[0] == nil {
		t.Errorf("group has nil child")
	}
}

// Shutdown

func ShutdownWrapTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withShutdown()
		st2 = withShutdown(st1)
		st3 = withShutdown(st2)

		okDone1 = runShutdownable(st1)
		okDone2 = runShutdownable(st2)
		okDone3 = runShutdownable(st3)
	)

	if isDone(
		st1.end, st2.end, st3.end,
		st1.done, st2.done, st3.done,
	) {
		t.Error(errInitClosed)
	}

	go st3.close()
	time.Sleep(failTimeout)

	switch {
	case isNotDone(st1.end):
		t.Error(errNotClosed)
	case isDone(st2.end, st3.end):
		t.Error(errClosed)
	case isDone(st1.done, st2.done, st3.done):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone1)

	switch {
	case isNotDone(st2.end):
		t.Error(errNotClosed)
	case isNotDone(st1.done):
		t.Error(errNotFinished)
	case isDone(st3.end):
		t.Error(errClosed)
	case isDone(st2.done, st3.done):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone2)

	switch {
	case isNotDone(st2.done):
		t.Error(errNotFinished)
	case isNotDone(st3.end):
		t.Error(errNotClosed)
	case isDone(st3.done):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone3)

	// st3 must be done
	if isNotDone(st3.done) {
		t.Error(errNotFinished)
	}
}

func ShutdownSuccessiveDoneTest(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("successive Done call caused panic")
		}
	}()

	var (
		st1 = withShutdown()

		okDone1 = runShutdownable(st1)
	)

	go st1.close()

	closeChanAndPropagate(okDone1)
	st1.Done()
}

func ShutdownSuccessiveCallTest(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("successive Shutdown call caused panic")
		}
	}()

	var (
		st1 = withShutdown()

		okDone1 = runShutdownable(st1)
	)

	go st1.close()
	closeChanAndPropagate(okDone1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := st1.Shutdown(ctx)
	if err != nil {
		t.Errorf(errTimeout)
	}

	err = st1.Shutdown(ctx)
	if err != nil {
		t.Errorf(errTimeout)
	}
}

func ShutdownTimeoutTest(t *testing.T) {
	t.Parallel()
	st := withShutdown()

	// blocked finish
	_ = runShutdownable(st)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := st.Shutdown(ctx)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("blocked shutdown didn't timeout")
	}
}

func ShutdownUnclosedTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withShutdown()
		st2 = withShutdown(st1)
	)

	okDone2 := runShutdownable(st2)
	okDone1 := runShutdownable(st1)

	closeChanAndPropagate(okDone1, okDone2)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := st2.Shutdown(ctx)
	if err != nil {
		t.Errorf(errTimeout)
	}

	if err := st1.cause(); err != nil {
		t.Errorf(errNotFinished)
	}

	if err := st2.cause(); err != nil {
		t.Errorf(errNotFinished)
	}
}

// Wait

func WaitTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withWait()
		st2 = withWait(st1)
		st3 = withWait(st2)

		okDone1 = runWaitable(st1)
		okDone2 = runWaitable(st2)
		okDone3 = runWaitable(st3)
	)

	done := make(chan struct{})

	go func() {
		st3.Wait()
		close(done)
	}()

	time.Sleep(failTimeout)

	if isDone(done) {
		t.Error(errNotWaited)
	}

	closeChanAndPropagate(okDone1, okDone2)

	if isDone(done) {
		t.Error(errNotWaited)
	}

	closeChanAndPropagate(okDone3)

	time.Sleep(failTimeout)

	if isNotDone(done) {
		t.Error(errFinishWaiting)
	}
}

// Value

type key string

func ValueWrapTest(t *testing.T) {
	t.Parallel()
	var (
		testKey   = key("test_key")
		testValue = "test_value"
		st1       = withValue(testKey, testValue)
		st2       = withWait(st1)
		st3       = withWait(st2)
	)

	value := st3.Value(testKey)
	if value == nil {
		t.Error("test value for test key not found")
	}

	valueTyped, ok := value.(string)
	if !ok {
		t.Error("test value for test key has wrong type")
	}

	if valueTyped != testValue {
		t.Errorf("wrong test value: want %s have %s", testValue, valueTyped)
	}
}

func ValueChildrenTest(t *testing.T) {
	t.Parallel()
	var (
		testKey1   = key("test_key1")
		testValue1 = "test_value2"

		testKey2   = key("test_key2")
		testValue2 = "test_value2"

		testKeyNotFound = key("test_key3")
		st1             = withValue(testKey1, testValue1)
		st2             = withValue(testKey2, testValue2, st1)
	)

	value := st2.Value(testKey1)
	if value == nil {
		t.Error("test value 1 for test key not found")
	}

	valueTyped, ok := value.(string)
	if !ok {
		t.Error("test value 1 for test key has wrong type")
	}

	if valueTyped != testValue1 {
		t.Errorf("wrong test value 1: want %s have %s", testValue1, valueTyped)
	}

	value2 := st2.Value(testKey2)
	if value2 == nil {
		t.Error("test value 2 for test key not found")
	}

	value2Typed, ok := value2.(string)
	if !ok {
		t.Error("test value 2 for test key has wrong type")
	}

	if value2Typed != testValue2 {
		t.Errorf("wrong test value 2: want %s have %s", testValue1, valueTyped)
	}

	value3 := st2.Value(testKeyNotFound)
	if value3 != nil {
		t.Error("unused key returned non-nil value")
	}
}

func ValueNilPanicTest(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("nil key did not panic")
		}
	}()

	_ = withValue(nil, "")
}

func ValueComparablePanicTest(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("incomparable key did not panic")
		}
	}()

	_ = withValue(func() {}, "")
}

// Annotate

func AnnotationErrorTest(t *testing.T) {
	t.Parallel()
	const annotation = "test"

	var (
		err1 = errors.New("error1")
		err2 = errors.New("error2")

		st1 = withError(err1)
		st2 = withError(err2, st1)
		st3 = withAnnotation(annotation, st2)
	)

	err := st3.Err()

	// err2 is higher in the tree so it must be found first
	if !errors.Is(err, err2) {
		t.Errorf("wrong error, want '%v', have '%v'", err2, err)
	}

	wantErrStr := fmt.Sprintf("%s: %s", annotation, err2.Error())

	if err.Error() != wantErrStr {
		t.Errorf("timeout error is not annotated, want error '%s', have '%s'", wantErrStr, err.Error())
	}

	err = st1.Err()
	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}
}

func AnnotationShutdownTimeoutTest(t *testing.T) {
	t.Parallel()
	const annotation = "test"

	st1 := withShutdown()
	st2 := withAnnotation(annotation, st1)

	// blocked finish
	_ = runShutdownable(st1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := st2.Shutdown(ctx)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("blocked shutdown didn't timeout")
	}

	wantErrStr := fmt.Sprintf("%s: %s", annotation, ErrTimeout.Error())

	if err.Error() != wantErrStr {
		t.Errorf("timeout error is not annotated, want error '%s', have '%s'", wantErrStr, err.Error())
	}
}

func AnnotationChildShutdownTimeoutTest(t *testing.T) {
	t.Parallel()
	const annotation = "test"

	st1 := withShutdown()
	st2 := withAnnotation(annotation, st1)
	st3 := withShutdown(st2)

	// blocked finish
	_ = runShutdownable(st1)
	_ = runShutdownable(st3)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := st3.Shutdown(ctx)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("blocked shutdown didn't timeout")
	}

	wantErrStr := fmt.Sprintf("%s: %s", annotation, ErrTimeout.Error())

	if err.Error() != wantErrStr {
		t.Errorf("timeout error is not annotated, want error '%s', have '%s'", wantErrStr, err.Error())
	}
}

func AnnotationNilErrorTest(t *testing.T) {
	t.Parallel()
	const message = "test"

	st1 := withError(nil)
	st2 := withAnnotation(message, st1)

	if err := st2.Err(); err != nil {
		t.Errorf("annotated state returned error instead of nil")
	}
}

func AnnotationNilShutdownErrorTest(t *testing.T) {
	t.Parallel()
	st1 := withShutdown()
	st2 := withAnnotation("", st1)

	// blocked finish
	okDone1 := runShutdownable(st1)

	closeChanAndPropagate(okDone1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	if err := st2.Shutdown(ctx); err != nil {
		t.Errorf("annotation state returned error instead of nil")
	}
}

func AnnotationUnclosedTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withShutdown()
		st2 = withAnnotation("", st1)
	)

	okDone1 := runShutdownable(st1)

	closeChanAndPropagate(okDone1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := st2.Shutdown(ctx)
	if err != nil {
		t.Errorf(errTimeout)
	}

	if err := st1.cause(); err != nil {
		t.Errorf(errNotFinished)
	}

	if err := st2.cause(); err != nil {
		t.Errorf(errNotFinished)
	}
}

// Error

func ErrorTest(t *testing.T) {
	t.Parallel()
	var (
		err1 = errors.New("error1")
		err2 = errors.New("error2")

		st1 = withError(err1)
		st2 = withError(err2, st1)
	)

	err := st2.Err()

	// err2 is higher in the tree so it must be found first
	if !errors.Is(err, err2) {
		t.Errorf("wrong error, want '%v', have '%v'", err2, err)
	}

	err = st1.Err()
	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}
}

// Error group

func ErrorGroupTest(t *testing.T) {
	t.Parallel()
	const annotation = "test"

	var (
		err1 = errors.New("error1")
		err2 = errors.New("error2")
		st1  = withErrorGroup()
		st2  = withAnnotation(annotation, st1)
	)

	if err := st2.Err(); err != nil {
		t.Errorf("new error group state returned error")
	}

	go func() {
		st1.Error(err1)
	}()

	time.Sleep(failTimeout)

	err := st2.Err()

	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}

	wantErrStr := fmt.Sprintf("%s: %s", annotation, err1.Error())

	if err.Error() != wantErrStr {
		t.Errorf("error group error is not annotated, want '%s', have '%s'", wantErrStr, err.Error())
	}

	go func() {
		st1.Error(err2)
	}()

	time.Sleep(failTimeout)

	err = st2.Err()

	if errors.Is(err, err2) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err2)
	}
}

func ErrorGroupErrorfTest(t *testing.T) {
	t.Parallel()
	const annotation = "test: %w"

	var (
		err1 = errors.New("error1")
		err2 = errors.New("error2")
		st1  = withErrorGroup()
		st2  = merge(st1)
	)

	if err := st2.Err(); err != nil {
		t.Errorf("new error group state returned error")
	}

	go func() {
		st1.Errorf(annotation, err1)
	}()

	time.Sleep(failTimeout)

	err := st2.Err()

	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}

	wantErrStr := fmt.Sprintf("%s: %s", "test", err1.Error())

	if err.Error() != wantErrStr {
		t.Errorf("error group error is not annotated, want '%s', have '%s'", wantErrStr, err.Error())
	}

	go func() {
		st1.Errorf(annotation, err2)
	}()

	time.Sleep(failTimeout)

	err = st2.Err()

	if errors.Is(err, err2) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err2)
	}
}

// Empty

func EmptyTest(t *testing.T) {
	t.Parallel()
	st1 := emptyState{}

	if err := st1.Err(); err != nil {
		t.Errorf("empty state returned error")
	}

	if err := st1.Shutdown(context.Background()); err != nil {
		t.Errorf("empty state shutdowned with error")
	}

	okDone1 := make(chan struct{})
	go func() {
		st1.Wait()
		close(okDone1)
	}()

	time.Sleep(failTimeout)

	if isNotDone(okDone1) {
		t.Errorf("empty state blocked on wait")
	}

	if value := st1.Value(""); value != nil {
		t.Errorf("empty state returned value")
	}

	okDone2 := make(chan struct{})
	go func() {
		st1.close()
		close(okDone2)
	}()

	time.Sleep(failTimeout)

	if isNotDone(okDone2) {
		t.Errorf("empty state blocked on close")
	}

	if isNotDone(st1.finishSig()) {
		t.Errorf("empty state is not done")
	}

	if err := st1.cause(); err != nil {
		t.Errorf("empty state cause call returned error")
	}
}

// Dependency

func DependencyShutdownTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withShutdown()
		st2 = withShutdown()
		st3 = withShutdown()

		okDone1 = runShutdownable(st1)
		okDone2 = runShutdownable(st2)
		okDone3 = runShutdownable(st3)
	)

	st4 := withDependency(st3, st1, st2)

	if isDone(
		st1.end, st2.end, st3.end,
		st1.done, st2.done, st3.done,
	) {
		t.Error(errInitClosed)
	}

	go st4.close()
	time.Sleep(failTimeout)

	switch {
	case isNotDone(st1.end, st2.end):
		t.Error(errNotClosed)
	case isDone(st3.end):
		t.Error(errClosed)
	case isDone(st1.done, st2.done, st3.done):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone1, okDone2)

	switch {
	case isNotDone(st3.end):
		t.Error(errNotClosed)
	case isNotDone(st1.done, st2.done):
		t.Error(errNotFinished)
	case isDone(st3.done):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone3)

	if isNotDone(st3.done) {
		t.Error(errNotFinished)
	}
}

func DependencyShutdownChainTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withShutdown()
		st2 = withShutdown()
		st3 = withShutdown()

		okDone1 = runShutdownable(st1)
		okDone2 = runShutdownable(st2)
		okDone3 = runShutdownable(st3)
	)

	st4 := withDependency(st3, st1)
	st4 = withDependency(st4, st2)

	if isDone(
		st1.end, st2.end, st3.end,
		st1.done, st2.done, st3.done, st4.finished,
	) {
		t.Error(errInitClosed)
	}

	go st4.close()
	time.Sleep(failTimeout)

	switch {
	case isNotDone(st2.end):
		t.Error(errNotClosed)
	case isDone(st1.end, st3.end):
		t.Error(errClosed)
	case isDone(st1.done, st2.done, st3.done, st4.finished):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone2)

	switch {
	case isNotDone(st1.end):
		t.Error(errNotClosed)
	case isNotDone(st2.done):
		t.Error(errNotFinished)
	case isDone(st3.end):
		t.Error(errClosed)
	case isDone(st1.done, st3.done, st4.finished):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone1)

	switch {
	case isNotDone(st3.end):
		t.Error(errNotClosed)
	case isNotDone(st1.done):
		t.Error(errNotFinished)
	case isDone(st3.done, st4.finished):
		t.Error(errFinished)
	}

	closeChanAndPropagate(okDone3)

	if isNotDone(st3.done, st4.finished) {
		t.Error(errNotFinished)
	}
}

func DependencyShutdownSuccessiveCloseTest(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("successive close call caused panic")
		}
	}()

	var (
		st1 = withShutdown()
		st2 = withShutdown()
	)

	close(runShutdownable(st1))
	close(runShutdownable(st2))

	st4 := withDependency(st1, st2)

	go st4.close()
	time.Sleep(failTimeout)

	st4.close()
}

func DependencyShutdownChildrenTimeoutTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withShutdown()
		st2 = emptyState{}
		st3 = st1.DependsOn(st2)
	)

	// blocked finish
	_ = runShutdownable(st1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := st3.Shutdown(ctx)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("blocked shutdown didn't timeout")
	}
}

func DependencyShutdownParentTimeoutTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withShutdown()
		st2 = withShutdown()
		st3 = st1.DependsOn(st2)
	)

	okDone1 := runShutdownable(st1)
	_ = runShutdownable(st2)

	closeChanAndPropagate(okDone1)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := st3.Shutdown(ctx)
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("blocked shutdown didn't timeout")
	}
}

func DependencyShutdownUnclosedTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withShutdown()
		st2 = withShutdown()
		st3 = withDependency(st1, st2)
	)

	// blocked finish
	okDone2 := runShutdownable(st2)
	okDone1 := runShutdownable(st1)

	closeChanAndPropagate(okDone1, okDone2)

	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()

	err := st3.Shutdown(ctx)
	if err != nil {
		t.Errorf(errTimeout)
	}

	if err := st3.cause(); err != nil {
		t.Errorf(errNotFinished)
	}
}

func DependencyWaitTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withWait()
		st2 = withWait()
		st3 = withWait()

		okDone1 = runWaitable(st1)
		okDone2 = runWaitable(st2)
		okDone3 = runWaitable(st3)

		st4 = st3.DependsOn(st1, st2)
	)

	done := make(chan struct{})

	go func() {
		st4.Wait()
		close(done)
	}()

	time.Sleep(failTimeout)

	if isDone(done) {
		t.Error(errNotWaited)
	}

	closeChanAndPropagate(okDone1, okDone2)

	if isDone(done) {
		t.Error(errNotWaited)
	}

	closeChanAndPropagate(okDone3)

	if isNotDone(done) {
		t.Error(errNotWaited)
	}
}

func DependencyErrorParentTest(t *testing.T) {
	t.Parallel()
	var (
		err1 = errors.New("error1")
		err2 = errors.New("error2")

		st1 = withError(err1)
		st2 = withError(err2)
		st3 = withDependency(st1, st2)
	)

	err := st3.Err()

	// err1 is higher in the tree so it must be found first
	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}

	err = st2.Err()
	if !errors.Is(err, err2) {
		t.Errorf("wrong error, want '%v', have '%v'", err2, err)
	}
}

func DependencyErrorChildrenTest(t *testing.T) {
	t.Parallel()
	var (
		err1 = errors.New("error1")

		st1 = withError(err1)
		st2 = emptyState{}
		st3 = st2.DependsOn(st1)
	)

	err := st3.Err()

	if !errors.Is(err, err1) {
		t.Errorf("wrong error, want '%v', have '%v'", err1, err)
	}
}

func DependencyErrorNilTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = withError(nil)
		st2 = emptyState{}
		st3 = st2.DependsOn(st1)
	)

	if err := st3.Err(); err != nil {
		t.Errorf("error must be nil, have '%v'", err)
	}
}

func DependencyValueParentTest(t *testing.T) {
	t.Parallel()
	var (
		testKey   = key("test_key")
		testValue = "test_value"
		st1       = emptyState{}
		st2       = emptyState{}
		st3       = st2.DependsOn(st1)
		st4       = withValue(testKey, testValue)
		st5       = withDependency(st4, st3)
	)

	value := st5.Value(testKey)
	if value == nil {
		t.Error("test value for test key not found")
	}

	valueTyped, ok := value.(string)
	if !ok {
		t.Error("test value for test key has wrong type")
	}

	if valueTyped != testValue {
		t.Errorf("wrong test value: want %s have %s", testValue, valueTyped)
	}
}

func DependencyValueChildrenTest(t *testing.T) {
	t.Parallel()
	var (
		testKey   = key("test_key")
		testValue = "test_value"
		st1       = withValue(testKey, testValue)
		st2       = emptyState{}
		st3       = st2.DependsOn(st1)
		st4       = emptyState{}
		st5       = st4.DependsOn(st3)
	)

	value := st5.Value(testKey)
	if value == nil {
		t.Error("test value for test key not found")
	}

	valueTyped, ok := value.(string)
	if !ok {
		t.Error("test value for test key has wrong type")
	}

	if valueTyped != testValue {
		t.Errorf("wrong test value: want %s have %s", testValue, valueTyped)
	}
}

func DependencyAnnotationTest(t *testing.T) {
	t.Parallel()
	var (
		st1 = emptyState{}
		st2 = emptyState{}
		st3 = withAnnotation("", st1, st2)
		st4 = emptyState{}
		st5 = withDependency(st3, st4)
	)

	if st5.parent == nil {
		t.Errorf("parent of dependency state is nil, must be empty state")
	}

	if st5.parent != st3 {
		t.Errorf("wrong parent of dependency state")
	}

	if len(st5.children.states) != 1 {
		t.Errorf("wrong number of dependency state children, want 1, have %d", len(st5.children.states))
		return
	}

	if st5.children.states[0] != st4 {
		t.Errorf("wrong children of dependency state")
	}
}
