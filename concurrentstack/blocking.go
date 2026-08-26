package concurrentstack

import (
	"context"
	"iter"

	"github.com/wow-look-at-my/go-containers/internal/blocking"
)

// ErrCompleted is what a push or pop returns after CompleteAdding; shared with BlockingList/BlockingBag.
var ErrCompleted = blocking.ErrCompleted

// Unbounded is the capacity of a stack that never makes a push wait.
const Unbounded = blocking.Unbounded

// BlockingStack is a Stack whose Push/Pop wait on full/empty, bounded by ctx.
// Zero value not usable -- use NewBlocking.
type BlockingStack[T any] struct {
	stack *Stack[T]
	core  *blocking.Core[T]
}

// BlockingOption configures a BlockingStack.
type BlockingOption func(*blockingConfig)

type blockingConfig struct {
	capacity int
}

// WithCapacity bounds the stack to n values, so Push waits once it fills. Zero or below is unbounded.
func WithCapacity(n int) BlockingOption {
	return func(c *blockingConfig) { c.capacity = n }
}

// NewBlocking creates an empty blocking stack. Without WithCapacity the stack
// is unbounded, and a push never waits.
func NewBlocking[T any](opts ...BlockingOption) *BlockingStack[T] {
	cfg := blockingConfig{capacity: Unbounded}
	for _, opt := range opts {
		opt(&cfg)
	}
	s := New[T]()
	return &BlockingStack[T]{stack: s, core: blocking.NewCore[T](s, cfg.capacity)}
}

// Push waits while a bounded stack is full; ErrCompleted after CompleteAdding, or ctx.Err().
func (b *BlockingStack[T]) Push(ctx context.Context, value T) error {
	return b.core.Add(ctx, value)
}

// TryPush adds value without waiting; false when full or complete.
func (b *BlockingStack[T]) TryPush(value T) bool { return b.core.TryAdd(value) }

// Pop removes the top value, waiting while the stack is empty; ErrCompleted once complete and empty, or ctx.Err().
func (b *BlockingStack[T]) Pop(ctx context.Context) (T, error) { return b.core.Take(ctx) }

// TryPop removes the top value without waiting; false when the stack is empty.
func (b *BlockingStack[T]) TryPop() (T, bool) { return b.core.TryTake() }

// Consume removes values, newest first, until the stack completes and empties, or ctx ends.
func (b *BlockingStack[T]) Consume(ctx context.Context) iter.Seq[T] { return b.core.Consume(ctx) }

// CompleteAdding marks the stack complete and wakes every waiter; they drain what's left, then see ErrCompleted.
func (b *BlockingStack[T]) CompleteAdding() { b.core.CompleteAdding() }

// IsAddingCompleted reports whether CompleteAdding ran.
func (b *BlockingStack[T]) IsAddingCompleted() bool { return b.core.IsAddingCompleted() }

// IsCompleted reports complete-and-empty; true means no consumer gets another value.
func (b *BlockingStack[T]) IsCompleted() bool { return b.core.IsCompleted() }

// Len is a reading; another goroutine can change it before the caller acts.
func (b *BlockingStack[T]) Len() int { return b.core.Len() }

// IsEmpty reports whether the stack holds no value that a pop can remove.
func (b *BlockingStack[T]) IsEmpty() bool { return b.core.IsEmpty() }

// Cap returns the bounded capacity, or Unbounded.
func (b *BlockingStack[T]) Cap() int { return b.core.Cap() }

// TryPeek returns the top value without removing it; false when the stack is empty.
func (b *BlockingStack[T]) TryPeek() (T, bool) { return b.stack.TryPeek() }

// All iterates the values, top first, removing none; same best-effort reading as Stack.All.
func (b *BlockingStack[T]) All() iter.Seq[T] { return b.stack.All() }

// Values returns the values as a slice, top first, and removes none of them.
func (b *BlockingStack[T]) Values() []T { return b.stack.Values() }
