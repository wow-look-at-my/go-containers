package blocking

import (
	"context"
	"sync"
)

// waiter is one goroutine parked on a semaphore. ready closes when the waiter
// receives a permit, or when the semaphore completes.
//
// Each park makes a fresh channel, and a pool must not recycle them.
// testing/synctest binds a channel to the bubble that created it, and it stops
// the program when another bubble touches that channel. A shared pool would
// therefore break the synctest tests of any caller of this library.
type waiter struct {
	ready   chan struct{}
	granted bool
	prev    *waiter
	next    *waiter
}

// sema is a counting semaphore that a context can cancel.
//
// A channel alone cannot do this job: the permit count of an unbounded
// collection has no ceiling, so there is no buffer size to give it. The waiters
// form a queue, so a permit goes to the goroutine that waited longest.
type sema struct {
	mu        sync.Mutex
	permits   int
	completed bool
	head      *waiter
	tail      *waiter
}

func (s *sema) enqueue(w *waiter) {
	w.prev = s.tail
	if s.tail == nil {
		s.head = w
	} else {
		s.tail.next = w
	}
	s.tail = w
}

// remove unlinks w and reports whether w was still in the queue.
func (s *sema) remove(w *waiter) bool {
	if w.prev == nil && w.next == nil && s.head != w {
		return false
	}
	if w.prev == nil {
		s.head = w.next
	} else {
		w.prev.next = w.next
	}
	if w.next == nil {
		s.tail = w.prev
	} else {
		w.next.prev = w.prev
	}
	w.prev, w.next = nil, nil
	return true
}

// release adds one permit, and hands it straight to the longest waiter when
// one is parked.
func (s *sema) release() {
	s.mu.Lock()
	if w := s.head; w != nil {
		s.remove(w)
		w.granted = true
		s.mu.Unlock()
		close(w.ready)
		return
	}
	s.permits++
	s.mu.Unlock()
}

// tryAcquire takes one permit without any wait.
func (s *sema) tryAcquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.permits > 0 {
		s.permits--
		return true
	}
	return false
}

// acquire takes one permit. It reports false when ctx ends first, and false
// when the semaphore completes with no permit left.
//
// A permit already handed to this waiter wins over a cancelled context. The
// permit stands for one element or one free slot, and a caller that drops it
// would lose that element or that slot.
func (s *sema) acquire(ctx context.Context) bool {
	s.mu.Lock()
	if s.permits > 0 {
		s.permits--
		s.mu.Unlock()
		return true
	}
	if s.completed {
		s.mu.Unlock()
		return false
	}
	if ctx.Err() != nil {
		s.mu.Unlock()
		return false
	}
	w := &waiter{ready: make(chan struct{})}
	s.enqueue(w)
	s.mu.Unlock()

	// A context that can never end has a nil channel. One receive costs much
	// less than a select, and this is the common case.
	done := ctx.Done()
	if done == nil {
		<-w.ready
		return w.granted
	}

	select {
	case <-w.ready:
		return w.granted
	case <-done:
		s.mu.Lock()
		removed := s.remove(w)
		s.mu.Unlock()
		if removed {
			return false
		}
		// A releaser already took this waiter out of the queue, so it
		// grants the permit. Wait for that to land.
		<-w.ready
		return w.granted
	}
}

// complete wakes every waiter and stops any further wait. Permits that are
// already there stay there, so a taker can still drain them with tryAcquire.
func (s *sema) complete() {
	s.mu.Lock()
	if s.completed {
		s.mu.Unlock()
		return
	}
	s.completed = true
	// Unlink every waiter under the lock. A waiter that cancels at the same
	// moment reads the same links from remove, which holds the same lock.
	var woken []*waiter
	for w := s.head; w != nil; {
		next := w.next
		w.prev, w.next = nil, nil
		woken = append(woken, w)
		w = next
	}
	s.head, s.tail = nil, nil
	s.mu.Unlock()

	for _, w := range woken {
		close(w.ready)
	}
}

// available returns the current permit count.
func (s *sema) available() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.permits
}
