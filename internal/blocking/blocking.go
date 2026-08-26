// Package blocking holds the bounding and completion core that the
// BlockingList, BlockingStack and BlockingBag types share.
//
// The core owns every wait. The store under it stays lock-free, and the core
// never holds a lock while it touches that store.
package blocking

import (
	"context"
	"errors"
	"iter"
	"runtime"
	"sync"
	"sync/atomic"
)

// ErrCompleted is what a blocked add or take returns after CompleteAdding; a take also needs it empty.
var ErrCompleted = errors.New("go-containers: the collection is complete for adds")

// Unbounded is the capacity of a collection that never makes an add wait.
const Unbounded = -1

// Store is concurrent-safe and non-blocking; TryTake may report false mid-add, so the core retries.
type Store[T any] interface {
	TryAdd(v T) bool
	TryTake() (T, bool)
	Len() int
}

// Core adds bounding, blocking and completion to a Store.
type Core[T any] struct {
	store Store[T]
	// items holds one permit per removable element.
	items sema
	// free holds one permit per empty slot; unused when unbounded.
	free     sema
	capacity int

	// completeMu: adds hold it for reading; CompleteAdding takes it for writing.
	completeMu sync.RWMutex
	completed  atomic.Bool
}

// NewCore wraps store. A capacity of Unbounded, or of any value below zero,
// means that an add never waits.
func NewCore[T any](store Store[T], capacity int) *Core[T] {
	c := &Core[T]{store: store, capacity: capacity}
	if capacity > 0 {
		c.free.permits = capacity - store.Len()
	}
	for range store.Len() {
		c.items.release()
	}
	return c
}

// Cap returns the bounded capacity, or Unbounded.
func (c *Core[T]) Cap() int {
	if c.capacity <= 0 {
		return Unbounded
	}
	return c.capacity
}

// Len returns the number of elements the collection holds.
func (c *Core[T]) Len() int { return c.store.Len() }

// IsEmpty reports whether the collection holds no elements.
func (c *Core[T]) IsEmpty() bool { return c.items.available() == 0 }

// IsAddingCompleted reports whether CompleteAdding ran.
func (c *Core[T]) IsAddingCompleted() bool { return c.completed.Load() }

// IsCompleted reports whether CompleteAdding ran and the collection is empty.
// A taker that sees true will never receive another element.
func (c *Core[T]) IsCompleted() bool {
	return c.completed.Load() && c.items.available() == 0
}

// CompleteAdding marks the collection complete and wakes every blocked
// goroutine; safe to call more than once. An add already in flight finishes.
func (c *Core[T]) CompleteAdding() {
	c.completeMu.Lock()
	already := c.completed.Swap(true)
	c.completeMu.Unlock()
	if already {
		return
	}
	c.items.complete()
	c.free.complete()
}

// add puts value into the store and hands one permit to a taker. The caller
// owns a free permit when the collection is bounded.
func (c *Core[T]) add(value T) error {
	c.completeMu.RLock()
	if c.completed.Load() {
		c.completeMu.RUnlock()
		c.releaseFree()
		return ErrCompleted
	}
	ok := c.store.TryAdd(value)
	c.completeMu.RUnlock()
	if !ok {
		c.releaseFree()
		return ErrCompleted
	}
	c.items.release()
	return nil
}

func (c *Core[T]) releaseFree() {
	if c.capacity > 0 {
		c.free.release()
	}
}

// Add puts value into the collection and waits for a free slot when the
// collection is full.
//
// It returns ErrCompleted after CompleteAdding, and ctx.Err() when ctx ends
// first.
func (c *Core[T]) Add(ctx context.Context, value T) error {
	if c.capacity > 0 && !c.free.acquire(ctx) {
		if err := ctx.Err(); err != nil {
			return err
		}
		return ErrCompleted
	}
	return c.add(value)
}

// TryAdd puts value into the collection without any wait. It reports false
// when the collection is full or complete for adds.
func (c *Core[T]) TryAdd(value T) bool {
	if c.capacity > 0 && !c.free.tryAcquire() {
		return false
	}
	return c.add(value) == nil
}

// take removes one element the caller's permit guarantees; it retries while
// another goroutine is still mid-add on it.
func (c *Core[T]) take() T {
	for {
		if v, ok := c.store.TryTake(); ok {
			c.releaseFree()
			return v
		}
		runtime.Gosched()
	}
}

// Take removes and returns one element, and waits for one when the collection
// is empty.
//
// It returns ErrCompleted when the collection is complete for adds and empty,
// and ctx.Err() when ctx ends first. A take that already received its element
// returns that element, even when ctx ended at the same moment.
func (c *Core[T]) Take(ctx context.Context) (T, error) {
	if c.items.acquire(ctx) {
		return c.take(), nil
	}
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	// CompleteAdding woke this taker. The collection can still hold
	// elements, and a taker drains them before it sees ErrCompleted.
	if c.items.tryAcquire() {
		return c.take(), nil
	}
	return zero, ErrCompleted
}

// TryTake removes and returns one element without any wait. It reports false
// when the collection is empty.
func (c *Core[T]) TryTake() (T, bool) {
	if !c.items.tryAcquire() {
		var zero T
		return zero, false
	}
	return c.take(), true
}

// Consume returns an iterator that removes elements until the collection is
// complete and empty, or until ctx ends.
func (c *Core[T]) Consume(ctx context.Context) iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			v, err := c.Take(ctx)
			if err != nil {
				return
			}
			if !yield(v) {
				return
			}
		}
	}
}
