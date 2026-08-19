package helper

import (
	"sync/atomic"
	"testing"
)

func TestDone(t *testing.T) {
	done, stop := Done()
	go func(done <-chan bool) {
		for <-done {
			t.Log(
				"done",
			)
		}
	}(done)

	stop()
}

func TestPipe(t *testing.T) {
	done, stop := Done()
	defer stop()
	ch := Pipe(done, func() int {
		return 1
	})

	for {
		select {
		case d := <-ch:
			t.Log("pipe", d)
			return
		case <-done:
			t.Log("done")
			return
		}
	}
}

func TestFanOut(t *testing.T) {
	done, stop := Done()
	defer stop()

	var i atomic.Int64

	fns := FanOut(done, func() int {
		return int(i.Add(1))
	}, 10)

	// for {
	// 	select {
	// 	case <-done:
	// 		t.Log("done TestFanOut")
	// 		return
	// 	default:
	// 		for _, fn := range fns {
	// 			d := <-fn
	// 			t.Log("fan out", d)
	// 		}
	// 		stop()
	// 	}

	// }

	for _, fn := range fns {
		t.Log("fan out", <-fn)
	}

	t.Log("fan out done")
}

func TestFanIn(t *testing.T) {
	done, stop := Done()
	defer stop()
	var i atomic.Int64

	fns := FanOut(done, func() int {
		return int(i.Add(1))
	}, 10)

	ch := FanIn(done, fns)

	for d := range ch {
		t.Log("fan in", d)
	}
}

func TestParallel(t *testing.T) {
	done, stop := Done()
	defer stop()

	fns := Parallel(
		done,
		func() error {
			t.Log("fn1")
			return nil
		},
		func() error {
			t.Log("fn2")
			return nil
		},
	)

	// for {
	// 	select {
	// 	case <-done:
	// 		t.Log("done TestParallel")
	// 		return
	// 	default:
	// 		for _, fn := range fns {
	// 			t.Log("parallel", <-fn)
	// 		}
	// 		return
	// 	}
	// }

	for _, fn := range fns {
		t.Log("parallel", <-fn)
	}

}
