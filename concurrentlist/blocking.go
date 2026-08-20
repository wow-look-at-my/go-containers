package concurrentlist

import (
	"context"
	"iter"

	"github.com/wow-look-at-my/go-containers/internal/blocking"
)

// ErrCompleted is the error an add or a take returns after CompleteAdding. A
// take returns it only after the list is also empty.
//
// The BlockingBag and BlockingStack types report the same error value, so one
// consumer can handle every blocking collection with one comparison.
var ErrCompleted = blocking.ErrCompleted

// Unbounded is the capacity of a list that never makes an append wait.
const Unbounded = blocking.Unbounded

// BlockingList is a List that can make a caller wait.
//
// An append waits while a bounded list is full. A take waits while the list is
// empty. The order stays the same as List: takes return the elements in the
// order the appends reserved them.
//
// Every wait ends on a context, so no caller can be stuck. Use NewBlocking to
// create one; the zero value is not usable.
type BlockingList[T any] struct {
	list *List[T]
	core *blocking.Core[T]
}

// BlockingOption configures a BlockingList.
type BlockingOption func(*blockingConfig)

type blockingConfig struct {
	capacity int
}

// WithCapacity bounds the list to n elements. An append then waits while the
// list is full, which stops the producers from running away from the
// consumers. A value of zero or below leaves the list unbounded.
func WithCapacity(n int) BlockingOption {
	return func(c *blockingConfig) { c.capacity = n }
}

// NewBlocking creates an empty blocking list. Without WithCapacity the list is
// unbounded, and an append never waits.
func NewBlocking[T any](opts ...BlockingOption) *BlockingList[T] {
	cfg := blockingConfig{capacity: Unbounded}
	for _, opt := range opts {
		opt(&cfg)
	}
	l := New[T]()
	return &BlockingList[T]{list: l, core: blocking.NewCore[T](l, cfg.capacity)}
}

// Append adds value to the end of the list, and waits while a bounded list is
// full.
//
// It returns ErrCompleted after CompleteAdding, and ctx.Err() when ctx ends
// first.
func (b *BlockingList[T]) Append(ctx context.Context, value T) error {
	return b.core.Add(ctx, value)
}

// TryAppend adds value without any wait. It reports false when the list is
// full or complete for appends.
func (b *BlockingList[T]) TryAppend(value T) bool { return b.core.TryAdd(value) }

// Take removes and returns the oldest element, and waits while the list is
// empty.
//
// It returns ErrCompleted when the list is complete for appends and empty, and
// ctx.Err() when ctx ends first.
func (b *BlockingList[T]) Take(ctx context.Context) (T, error) { return b.core.Take(ctx) }

// TryTake removes and returns the oldest element without any wait. It reports
// false when the list is empty.
func (b *BlockingList[T]) TryTake() (T, bool) { return b.core.TryTake() }

// Consume returns an iterator that removes elements, oldest first, until the
// list is complete and empty, or until ctx ends. It is the loop that a
// consumer goroutine wants.
func (b *BlockingList[T]) Consume(ctx context.Context) iter.Seq[T] { return b.core.Consume(ctx) }

// CompleteAdding marks the list complete for appends and wakes every waiting
// goroutine. Consumers drain what is left, and then see ErrCompleted.
func (b *BlockingList[T]) CompleteAdding() { b.core.CompleteAdding() }

// IsAddingCompleted reports whether CompleteAdding ran.
func (b *BlockingList[T]) IsAddingCompleted() bool { return b.core.IsAddingCompleted() }

// IsCompleted reports whether the list is complete for appends and empty. A
// consumer that sees true will never receive another element.
func (b *BlockingList[T]) IsCompleted() bool { return b.core.IsCompleted() }

// Len returns the number of elements in the list. The result is a reading:
// another goroutine can change it before the caller acts on it.
func (b *BlockingList[T]) Len() int { return b.core.Len() }

// IsEmpty reports whether the list holds no element that a take can remove.
func (b *BlockingList[T]) IsEmpty() bool { return b.core.IsEmpty() }

// Cap returns the bounded capacity, or Unbounded.
func (b *BlockingList[T]) Cap() int { return b.core.Cap() }

// All returns an iterator over the elements, oldest first, and removes none of
// them. It carries the same best-effort reading as List.All.
func (b *BlockingList[T]) All() iter.Seq[T] { return b.list.All() }

// Values returns the elements as a slice, oldest first, and removes none of
// them.
func (b *BlockingList[T]) Values() []T { return b.list.Values() }
