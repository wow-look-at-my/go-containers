package concurrentbag

import (
	"context"
	"iter"

	"github.com/wow-look-at-my/go-containers/internal/blocking"
)

// ErrCompleted is the error an add or a take returns after CompleteAdding. A
// take returns it only after the bag is also empty.
//
// The BlockingList and BlockingStack types report the same error value, so one
// consumer can handle every blocking collection with one comparison.
var ErrCompleted = blocking.ErrCompleted

// Unbounded is the capacity of a bag that never makes an add wait.
const Unbounded = blocking.Unbounded

// BlockingBag is a Bag that can make a caller wait.
//
// An add waits while a bounded bag is full. A take waits while the bag is
// empty. The bag keeps no order, so a take returns any element. Use
// BlockingList when the consumer needs the order of the adds, or BlockingStack
// when it needs the newest element first.
//
// Every wait ends on a context, so no caller can be stuck. Use NewBlocking to
// create one; the zero value is not usable.
type BlockingBag[T any] struct {
	bag  *Bag[T]
	core *blocking.Core[T]
}

// BlockingOption configures a BlockingBag.
type BlockingOption func(*blockingConfig)

type blockingConfig struct {
	capacity int
}

// WithCapacity bounds the bag to n elements. An add then waits while the bag
// is full, which stops the producers from running away from the consumers. A
// value of zero or below leaves the bag unbounded.
func WithCapacity(n int) BlockingOption {
	return func(c *blockingConfig) { c.capacity = n }
}

// NewBlocking creates an empty blocking bag. Without WithCapacity the bag is
// unbounded, and an add never waits.
//
// The bag under the waits shards by the default rule. WithConcurrency governs
// a plain Bag, and a blocking bag has no equivalent: the bound, not the shard
// count, is what decides its throughput.
func NewBlocking[T any](opts ...BlockingOption) *BlockingBag[T] {
	cfg := blockingConfig{capacity: Unbounded}
	for _, opt := range opts {
		opt(&cfg)
	}
	inner := New[T]()
	return &BlockingBag[T]{bag: inner, core: blocking.NewCore[T](inner, cfg.capacity)}
}

// Add puts value into the bag, and waits while a bounded bag is full.
//
// It returns ErrCompleted after CompleteAdding, and ctx.Err() when ctx ends
// first.
func (b *BlockingBag[T]) Add(ctx context.Context, value T) error {
	return b.core.Add(ctx, value)
}

// TryAdd puts value into the bag without any wait. It reports false when the
// bag is full or complete for adds.
func (b *BlockingBag[T]) TryAdd(value T) bool { return b.core.TryAdd(value) }

// Take removes and returns one element, and waits while the bag is empty. The
// bag keeps no order, so the element can be any of the ones it holds.
//
// It returns ErrCompleted when the bag is complete for adds and empty, and
// ctx.Err() when ctx ends first.
func (b *BlockingBag[T]) Take(ctx context.Context) (T, error) { return b.core.Take(ctx) }

// TryTake removes and returns one element without any wait. It reports false
// when the bag is empty.
func (b *BlockingBag[T]) TryTake() (T, bool) { return b.core.TryTake() }

// Consume returns an iterator that removes elements, in no order, until the
// bag is complete and empty, or until ctx ends. It is the loop that a consumer
// goroutine wants.
func (b *BlockingBag[T]) Consume(ctx context.Context) iter.Seq[T] { return b.core.Consume(ctx) }

// CompleteAdding marks the bag complete for adds and wakes every waiting
// goroutine. Consumers drain what is left, and then see ErrCompleted.
func (b *BlockingBag[T]) CompleteAdding() { b.core.CompleteAdding() }

// IsAddingCompleted reports whether CompleteAdding ran.
func (b *BlockingBag[T]) IsAddingCompleted() bool { return b.core.IsAddingCompleted() }

// IsCompleted reports whether the bag is complete for adds and empty. A
// consumer that sees true will never receive another element.
func (b *BlockingBag[T]) IsCompleted() bool { return b.core.IsCompleted() }

// Len returns the number of elements in the bag. The result is a reading:
// another goroutine can change it before the caller acts on it.
func (b *BlockingBag[T]) Len() int { return b.core.Len() }

// IsEmpty reports whether the bag holds no element that a take can remove.
func (b *BlockingBag[T]) IsEmpty() bool { return b.core.IsEmpty() }

// Cap returns the bounded capacity, or Unbounded.
func (b *BlockingBag[T]) Cap() int { return b.core.Cap() }

// TryPeek returns one element and leaves it in the bag. It reports false when
// the bag is empty.
func (b *BlockingBag[T]) TryPeek() (T, bool) { return b.bag.TryPeek() }

// All returns an iterator over the elements, in no order, and removes none of
// them. It carries the same best-effort reading as Bag.All.
func (b *BlockingBag[T]) All() iter.Seq[T] { return b.bag.All() }

// Values returns the elements as a slice, in no order, and removes none of
// them.
func (b *BlockingBag[T]) Values() []T { return b.bag.Values() }
