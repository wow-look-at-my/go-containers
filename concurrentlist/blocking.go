package concurrentlist

import (
	"context"
	"iter"

	"github.com/wow-look-at-my/go-containers/internal/blocking"
)

// ErrCompleted is what an add or take returns after CompleteAdding; shared with BlockingBag/BlockingStack.
var ErrCompleted = blocking.ErrCompleted

// Unbounded is the capacity of a list that never makes an append wait.
const Unbounded = blocking.Unbounded

// BlockingList is a List whose Append/Take wait on full/empty, bounded by ctx.
// Zero value not usable -- use NewBlocking.
type BlockingList[T any] struct {
	list *List[T]
	core *blocking.Core[T]
}

// BlockingOption configures a BlockingList.
type BlockingOption func(*blockingConfig)

type blockingConfig struct {
	capacity int
}

// WithCapacity bounds the list to n elements, so Append waits once it fills. Zero or below is unbounded.
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

// Append waits while a bounded list is full; ErrCompleted after CompleteAdding, or ctx.Err().
func (b *BlockingList[T]) Append(ctx context.Context, value T) error {
	return b.core.Add(ctx, value)
}

// TryAppend adds value without waiting; false when full or complete.
func (b *BlockingList[T]) TryAppend(value T) bool { return b.core.TryAdd(value) }

// Take removes the oldest element, waiting while the list is empty; ErrCompleted once complete and empty, or ctx.Err().
func (b *BlockingList[T]) Take(ctx context.Context) (T, error) { return b.core.Take(ctx) }

// TryTake removes the oldest element without waiting; false when the list is empty.
func (b *BlockingList[T]) TryTake() (T, bool) { return b.core.TryTake() }

// Consume removes elements, oldest first, until the list completes and empties, or ctx ends.
func (b *BlockingList[T]) Consume(ctx context.Context) iter.Seq[T] { return b.core.Consume(ctx) }

// CompleteAdding marks the list complete and wakes every waiter; they drain what's left, then see ErrCompleted.
func (b *BlockingList[T]) CompleteAdding() { b.core.CompleteAdding() }

// IsAddingCompleted reports whether CompleteAdding ran.
func (b *BlockingList[T]) IsAddingCompleted() bool { return b.core.IsAddingCompleted() }

// IsCompleted reports complete-and-empty; true means no consumer gets another element.
func (b *BlockingList[T]) IsCompleted() bool { return b.core.IsCompleted() }

// Len is a reading; another goroutine can change it before the caller acts.
func (b *BlockingList[T]) Len() int { return b.core.Len() }

// IsEmpty reports whether the list holds no element that a take can remove.
func (b *BlockingList[T]) IsEmpty() bool { return b.core.IsEmpty() }

// Cap returns the bounded capacity, or Unbounded.
func (b *BlockingList[T]) Cap() int { return b.core.Cap() }

// All iterates the elements, oldest first, removing none; same best-effort reading as List.All.
func (b *BlockingList[T]) All() iter.Seq[T] { return b.list.All() }

// Values returns the elements as a slice, oldest first, removing none.
func (b *BlockingList[T]) Values() []T { return b.list.Values() }
