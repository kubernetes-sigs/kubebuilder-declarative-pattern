package controllers

import "sync"

type Future[T any] struct {
	mutex  sync.Mutex
	cond   *sync.Cond
	done   bool
	result T
}

func newFuture[T any]() *Future[T] {
	f := &Future[T]{}
	f.cond = sync.NewCond(&f.mutex)
	return f
}

func (f *Future[T]) Set(value T) {
	f.mutex.Lock()
	if f.done {
		panic("Future::Set called more than once")
	}
	f.done = true
	f.result = value
	f.cond.Broadcast()
	f.mutex.Unlock()
}

func (f *Future[T]) Poll() (T, bool) {
	f.mutex.Lock()
	result, done := f.result, f.done
	f.mutex.Unlock()
	return result, done
}

func (f *Future[T]) Wait() T {
	f.mutex.Lock()
	for {
		if !f.done {
			f.cond.Wait()
		} else {
			result := f.result
			f.mutex.Unlock()
			return result
		}
	}
}
