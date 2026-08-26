# The concurrent collections

Five collections that many goroutines use at the same time, and a blocking
variant of three of them. All the synchronization is inside the type. No caller
of this library ever holds a lock, and no method returns a lock, a cursor, or
anything else that the caller must release.

## What each one is for

| type | order | mechanism |
| --- | --- | --- |
| `concurrentmap.Map[K, V]` | none | sharded maps, one `sync.RWMutex` per shard |
| `concurrentset.Set[T]` | none | `concurrentmap.Map[T, struct{}]` behind a set API |
| `concurrentlist.List[T]` | first in, first out | lock-free chain of contiguous segments |
| `concurrentstack.Stack[T]` | last in, first out | lock-free Treiber chain |
| `concurrentbag.Bag[T]` | none | sharded lock-free Treiber chains, with stealing |

## concurrentset: a set is a map that only needs its keys

`concurrentset.Set[T]` does not shard or lock anything itself -- every method
is one call into an internal `concurrentmap.Map[T, struct{}]`. That map has
already solved sharding and locking correctly, so a second implementation
would only be a second place for the same bugs to hide. `Add` is `TryAdd`,
`Remove` is `Delete`, `Contains`, `Len`, `IsEmpty`, `Clear`, and the `All`
iterator all forward directly. The type exists for the API, not the
mechanism: a caller who wants set semantics (unique elements, no value to
carry) should not have to spell out `Map[T, struct{}]` and remember that the
`struct{}` means nothing.

`concurrentmap` is the odd one: it uses locks, and it says so. A map needs a
hash table, and a lock-free hash table in Go costs an interface box or an
`unsafe.Pointer` per entry. The sharding is what removes the contention: two
goroutines that touch different shards never wait for each other.

## concurrentlist: the segment chain

A segment is one contiguous array of slots plus a `next` pointer. Contiguous
storage is the reason this is fast, and it is the choice moodycamel's
ConcurrentQueue makes for the same reason: a linked node per element costs one
allocation and one cache miss per element.

- An append reserves a slot with one atomic add on the segment tail. It then
  writes the value and stores the slot's ready flag.
- The ready flag is what makes the plain value field safe. The producer writes
  the value and then stores the flag. A consumer loads the flag and then reads
  the value. Go's memory model turns that pair into a happens-before edge, and
  the race detector agrees.
- A take reserves a slot with one compare-and-swap on the segment head. When
  the producer of that slot has not stored its flag yet, the taker spins. That
  wait ends after one store, because the taker already owns the slot.
- A full segment links a fresh one with a compare-and-swap, and the list tail
  moves onto it. Several producers can race here, and one of them wins.

Segment lengths start at 32 and double to 4096. A small first segment keeps an
empty list cheap, and a large later segment amortizes the allocation.

A take does NOT clear the slot it read. Clearing it would be a plain write to a
field that `All` can read at the same moment, which is a data race. The cost is
that a taken element stays referenced until its whole segment is released, so
at most one segment of elements.

`AppendRange` reserves a whole run with one atomic add, and raises the length
counter once for the run rather than once per element.

## concurrentstack and concurrentbag: why ABA cannot happen

Both are Treiber chains: push allocates a node and compare-and-swaps the top,
pop reads the top and compare-and-swaps it to `top.next`.

The classic defect of that structure is ABA. It cannot happen here, for a
reason that only holds in a garbage-collected language: these types never
recycle a node, and the goroutine that pops holds a live reference to the node
it read. The collector therefore cannot put a different node at that address
while the compare-and-swap runs, so the same pointer value can never stand for
a different node.

That argument breaks the moment a node comes back. Never add a free list, and
never add a `sync.Pool` of nodes. Either one brings ABA back.

The price of the argument is one allocation per push. It is why a mutex around
a slice wins the single-goroutine benchmarks: `append` writes 8 bytes into an
array that already exists. The bulk methods answer part of that. `PushRange`
and `AddRange` allocate the whole run as one `[]node[T]` and link it, so a run
of 64 costs one allocation rather than 64. The cost of that is that the last
surviving node of a run keeps the memory of the whole run.

### The bag has no thread affinity, and cannot have one

.NET's `ConcurrentBag<T>` is fast because each thread adds to and takes from
its own list. Go exposes no goroutine identity and no P identity to library
code, so that design is not available. The bag picks a shard with
`math/rand/v2`, whose top-level functions read the runtime's per-P generator,
so picking a shard costs no shared atomic. It spreads the contention; it does
not give locality. A take that finds its own shard empty steals from the
others, and reports false only after it has seen every shard empty.

## The blocking types

`BlockingList`, `BlockingStack` and `BlockingBag` are the same core over three
different stores. The core lives in `internal/blocking`, and the store contract
is the `Store[T]` interface: `TryAdd`, `TryTake`, `Len`. It is this repository's
`IProducerConsumerCollection<T>`.

Two semaphores carry the whole design. `items` holds one permit per element a
taker can remove. `free` holds one permit per empty slot, and a collection with
no bound never touches it. A take that holds a permit knows an element exists,
so it retries the store until the element appears rather than reporting empty.

The semaphore is a mutex, a counter and a queue of parked goroutines. A channel
cannot do the job on its own: the permit count of an unbounded collection has
no ceiling, so there is no buffer size to give it.

Each park allocates a channel, and a pool must not recycle them. `testing/synctest`
binds a channel to the bubble that created it and stops the program when
another bubble touches it, so a shared pool would break the synctest tests of
any caller of this library. That is not a theory: the pooled version failed
this repository's own synctest tests.

`CompleteAdding` takes the write side of a `sync.RWMutex` that every add holds
for reading. So when it returns, every add in flight has finished and no
further add can reach the store. It then wakes every parked goroutine. A woken
taker drains what is left before it sees `ErrCompleted`, which is what makes
"complete" and "empty" two separate states.

A cancelled wait that has already been handed a permit still succeeds. The
permit stands for one element or one free slot, and a caller that dropped it
would lose that element or that slot for good.

## Reading the benchmarks

Every `BenchmarkCompare*` runs the same workload on the library type and on
what a Go caller writes instead: a mutex around a map or a slice, `sync.Map`,
and a buffered channel. `p=N` is `b.SetParallelism(N)`, so N times GOMAXPROCS
goroutines.

The channel is not an equivalent of any of these types. It cannot be read
without removal, it has no bulk operation, it cannot grow past its buffer, and
it is first-in-first-out only. It is in the tables because it is what a Go
programmer reaches for.

Two rules kept the numbers honest, and both are worth keeping:

- **A fill benchmark builds a fresh collection per iteration.** An append-only
  benchmark on one shared collection holds every element it ever appends, and
  the framework raises the iteration count until that is gigabytes.
- **A take benchmark must put something back.** A collection cannot give more
  than it got, so a take-only steady state does not exist. Where a refill is
  inside the timer, the table says so.

Read the ratios, not the digits. The figures in README.md are medians of five
runs on one 4-vCPU sandbox. Single runs of the same code moved by up to 3x
here, which is why the headline claims rest on medians and not on one sample.
