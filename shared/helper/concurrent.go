package helper

import (
	"sync"
)

func Done() (<-chan bool, func(...error)) {
	done := make(chan bool)
	var once sync.Once

	go func() {
		for <-done {
			return
		}
	}()

	return done, func(errs ...error) {
		once.Do(func() {
			done <- true
			close(done)
		})
	}

}

func Pipe[T any](done <-chan bool, fn func() T) <-chan T {
	out := make(chan T)

	go func() {
		defer close(out)
		for {
			select {
			case <-done:
				return
			case out <- fn():
				return
			}
		}
	}()

	return out
}

func FanOut[T any](done <-chan bool, fn func() T, n int) []<-chan T {
	out := make([]<-chan T, n)

	for i := 0; i < n; i++ {
		out[i] = Pipe(done, fn)
	}

	return out
}

func FanIn[T any](done <-chan bool, in []<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup

	for _, ch := range in {
		wg.Add(1)
		go func(ch <-chan T) {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				case out <- <-ch:
					return
				}
			}
		}(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func Parallel(done <-chan bool, fns ...func() error) []<-chan error {
	out := make([]<-chan error, len(fns))

	for i := 0; i < len(fns); i++ {
		out[i] = Pipe(done, fns[i])
	}

	return out
}
