package concurrentstack

import (
	"context"
	"iter"

	"github.com/wow-look-at-my/go-containers/internal/blocking"
)

// ErrCompleted is the error a push or a pop returns after CompleteAdding. A
// pop returns it only after the stack is also empty.
//
// The BlockingList and BlockingBag types report the same error value, so one
// consumer can handle every blocking collection with one comparison.
var ErrCompleted = blocking.ErrCompleted

// Unbounded is the capacity of a stack that never makes a push wait.
const Unbounded = blocking.Unbounded

// BlockingStack is a Stack that can make a caller wait.
//
// A push waits while a bounded stack is full. A pop waits while the stack is
// empty. The order stays the same as Stack: the pop returns the value that was
// pushed last.
//
// Every wait ends on a context, so no caller can be stuck. Use NewBlocking to
// create one; the zero value is not usable.
type BlockingStack[T any] struct {
	stack *Stack[T]
	core  *blocking.Core[T]
}

// BlockingOption configures a BlockingStack.
type BlockingOption func(*blockingConfig)

type blockingConfig struct {
	capacity int
}

// WithCapacity bounds the stack to n values. A push then waits while the stack
// is full, which stops the producers from running away from the consumers. A
// value of zero or below leaves the stack unbounded.
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

// Push adds value to the top of the stack, and waits while a bounded stack is
// full.
//
// It returns ErrCompleted after CompleteAdding, and ctx.Err() when ctx ends
// first.
func (b *BlockingStack[T]) Push(ctx context.Context, value T) error {
	return b.core.Add(ctx, value)
}

// TryPush adds value without any wait. It reports false when the stack is full
// or complete for pushes.
func (b *BlockingStack[T]) TryPush(value T) bool { return b.core.TryAdd(value) }

// Pop removes and returns the value on top, and waits while the stack is
// empty.
//
// It returns ErrCompleted when the stack is complete for pushes and empty, and
// ctx.Err() when ctx ends first.
func (b *BlockingStack[T]) Pop(ctx context.Context) (T, error) { return b.core.Take(ctx) }

// TryPop removes and returns the value on top without any wait. It reports
// false when the stack is empty.
func (b *BlockingStack[T]) TryPop() (T, bool) { return b.core.TryTake() }

// Consume returns an iterator that removes values, newest first, until the
// stack is complete and empty, or until ctx ends. It is the loop that a
// consumer goroutine wants.
func (b *BlockingStack[T]) Consume(ctx context.Context) iter.Seq[T] { return b.core.Consume(ctx) }

// CompleteAdding marks the stack complete for pushes and wakes every waiting
// goroutine. Consumers drain what is left, and then see ErrCompleted.
func (b *BlockingStack[T]) CompleteAdding() { b.core.CompleteAdding() }

// IsAddingCompleted reports whether CompleteAdding ran.
func (b *BlockingStack[T]) IsAddingCompleted() bool { return b.core.IsAddingCompleted() }

// IsCompleted reports whether the stack is complete for pushes and empty. A
// consumer that sees true will never receive another value.
func (b *BlockingStack[T]) IsCompleted() bool { return b.core.IsCompleted() }

// Len returns the number of values in the stack. The result is a reading:
// another goroutine can change it before the caller acts on it.
func (b *BlockingStack[T]) Len() int { return b.core.Len() }

// IsEmpty reports whether the stack holds no value that a pop can remove.
func (b *BlockingStack[T]) IsEmpty() bool { return b.core.IsEmpty() }

// Cap returns the bounded capacity, or Unbounded.
func (b *BlockingStack[T]) Cap() int { return b.core.Cap() }

// TryPeek returns the value on top and leaves it in the stack. It reports
// false when the stack is empty.
func (b *BlockingStack[T]) TryPeek() (T, bool) { return b.stack.TryPeek() }

// All returns an iterator over the values, top first, and removes none of
// them. It carries the same best-effort reading as Stack.All.
func (b *BlockingStack[T]) All() iter.Seq[T] { return b.stack.All() }

// Values returns the values as a slice, top first, and removes none of them.
func (b *BlockingStack[T]) Values() []T { return b.stack.Values() }
