package concurrentbag

import (
	"context"
	"iter"

	"github.com/wow-look-at-my/go-containers/internal/blocking"
)

// ErrCompleted is what an add or take returns after CompleteAdding; shared with BlockingList/BlockingStack.
var ErrCompleted = blocking.ErrCompleted

// Unbounded is the capacity of a bag that never makes an add wait.
const Unbounded = blocking.Unbounded

// BlockingBag is a Bag whose Add/Take wait on full/empty, bounded by ctx.
// Zero value not usable -- use NewBlocking.
type BlockingBag[T any] struct {
	bag  *Bag[T]
	core *blocking.Core[T]
}

// BlockingOption configures a BlockingBag.
type BlockingOption func(*blockingConfig)

type blockingConfig struct {
	capacity int
}

// WithCapacity bounds the bag to n elements, so Add waits once it fills. Zero or below is unbounded.
func WithCapacity(n int) BlockingOption {
	return func(c *blockingConfig) { c.capacity = n }
}

// NewBlocking creates an empty blocking bag, unbounded without WithCapacity.
// The bound, not the shard count, decides its throughput -- there is no
// WithConcurrency equivalent here.
func NewBlocking[T any](opts ...BlockingOption) *BlockingBag[T] {
	cfg := blockingConfig{capacity: Unbounded}
	for _, opt := range opts {
		opt(&cfg)
	}
	inner := New[T]()
	return &BlockingBag[T]{bag: inner, core: blocking.NewCore[T](inner, cfg.capacity)}
}

// Add waits while a bounded bag is full; ErrCompleted after CompleteAdding, or ctx.Err().
func (b *BlockingBag[T]) Add(ctx context.Context, value T) error {
	return b.core.Add(ctx, value)
}

// TryAdd puts value in without waiting; false when full or complete.
func (b *BlockingBag[T]) TryAdd(value T) bool { return b.core.TryAdd(value) }

// Take removes any one element, waiting while the bag is empty; ErrCompleted once complete and empty, or ctx.Err().
func (b *BlockingBag[T]) Take(ctx context.Context) (T, error) { return b.core.Take(ctx) }

// TryTake removes one element without waiting; false when the bag is empty.
func (b *BlockingBag[T]) TryTake() (T, bool) { return b.core.TryTake() }

// Consume removes elements, in no order, until the bag completes and empties, or ctx ends.
func (b *BlockingBag[T]) Consume(ctx context.Context) iter.Seq[T] { return b.core.Consume(ctx) }

// CompleteAdding marks the bag complete and wakes every waiter; they drain what's left, then see ErrCompleted.
func (b *BlockingBag[T]) CompleteAdding() { b.core.CompleteAdding() }

// IsAddingCompleted reports whether CompleteAdding ran.
func (b *BlockingBag[T]) IsAddingCompleted() bool { return b.core.IsAddingCompleted() }

// IsCompleted reports complete-and-empty; true means no consumer gets another element.
func (b *BlockingBag[T]) IsCompleted() bool { return b.core.IsCompleted() }

// Len is a reading; another goroutine can change it before the caller acts.
func (b *BlockingBag[T]) Len() int { return b.core.Len() }

// IsEmpty reports whether the bag holds no element that a take can remove.
func (b *BlockingBag[T]) IsEmpty() bool { return b.core.IsEmpty() }

// Cap returns the bounded capacity, or Unbounded.
func (b *BlockingBag[T]) Cap() int { return b.core.Cap() }

// TryPeek returns one element without removing it; false when the bag is empty.
func (b *BlockingBag[T]) TryPeek() (T, bool) { return b.bag.TryPeek() }

// All iterates the elements, in no order, removing none; same best-effort reading as Bag.All.
func (b *BlockingBag[T]) All() iter.Seq[T] { return b.bag.All() }

// Values returns the elements as a slice, in no order, removing none.
func (b *BlockingBag[T]) Values() []T { return b.bag.Values() }
