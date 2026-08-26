# CLAUDE.md

## Build & Test

Run `go-toolchain` (no arguments) in the repository root. It tidies, vets,
tests with coverage, and builds. Never run a bare `go` command.

## Project Structure

- `set/set.go` — `Set[T comparable]`, a map with `struct{}` values behind an API. `Add` reports whether the element was new from the length before
  and after ONE insert, never a lookup plus an insert; `Union` sizes the result for both sides up front rather than cloning the larger and growing it.
  `Clone` copies with a range loop, which measured faster than `maps.Clone` for an empty-struct map. The zero value is an empty set, so every mutating method takes a
  pointer receiver and creates the map on first use; the read-only and algebraic methods take a value receiver and read a nil map fine. Union,
  Intersection, and IsDisjoint iterate the SMALLER set on purpose, and RemoveSet picks its side the same way.
- `set/json.go` — a set marshals to a JSON array of its elements and unmarshals from one, replacing whatever it held. An empty array leaves the map
  nil rather than allocating.
- `sortedmap/sortedmap.go` — `SortedMap[K, V]`, a left-leaning red-black tree. Keys stay in order at all times, so Put, Get, Delete, Min, Max, Floor
  and Ceiling are O(log n) and no walk ever sorts. `New` orders with `cmp.Compare`; `NewWithCompare` takes a comparison function for a key type with
  no natural order. The zero value is NOT usable — the comparison function is nil. Iterators: All, Keys, Values, Backward, and the half-open Range.
- `event/event.go` — `Event[T EventArgs]` and `ResultEvent[T EventArgs, R any]`, thread-safe dispatchers whose callbacks are WEAK pointers, so an
  event never keeps a dead subscriber alive. Event callbacks are `*func(T) error`; ResultEvent callbacks are `*func(T) (R, error)` and Invoke
  returns every live value plus the joined errors. The caller must retain the function pointer. Invoke calls every live callback even after one
  fails, joins the errors, and drops the callbacks whose referents are gone. T must embed `event.Args`, which is what stops a bare `int` argument
  that could never gain a field. Event.Invoke never allocates: one subscriber is copied to the stack and its error returned unwrapped, and more
  than one rides a pooled buffer. Both copy the callbacks and release the lock BEFORE calling any of them, which is what lets a callback
  subscribe or unsubscribe.
- `event/dispatcher.go` — private `dispatcher[CB]`, the weak set and snapshot pool both event types share.
- `concurrentmap/concurrentmap.go` — `Map[K, V]`, keys sharded across independently locked partitions, after .NET's ConcurrentDictionary.
  `hash/maphash.Comparable` picks the shard; each shard is padded to 128 bytes so two never share a cache line. The zero value is NOT
  usable and every method says so with a panic that names New. The callbacks of LoadOrCompute, AddOrUpdate and Compute run UNDER the
  shard lock, so each runs exactly once and the operation is atomic — .NET runs its delegate outside the lock and cannot promise either.
- `concurrentset/concurrentset.go` — `Set[T]`, a thin wrapper over `concurrentmap.Map[T, struct{}]`: every method is one call into the
  map, so the sharding and locking live in exactly one place. `New` forwards `concurrentmap`'s own options.
- `concurrentlist/concurrentlist.go` — `List[T]`, a lock-free first-in-first-out collection over a chain of contiguous segments. An
  append reserves one slot with one atomic add; a per-slot ready flag publishes the value and is what makes the plain value field
  race-free. A take never clears its slot, because that write would race with All. The zero value is an empty list.
- `concurrentstack/concurrentstack.go` — `Stack[T]`, a lock-free Treiber stack. `concurrentbag/concurrentbag.go` — `Bag[T]`, sharded
  Treiber chains with stealing, shard picked by `math/rand/v2` because Go exposes no P identity. Neither ever recycles a node, which is
  the whole ABA argument: never add a free list or a node pool. PushRange and AddRange allocate a run as ONE `[]node[T]`.
- `queue/queue.go` — `Queue[T]`, a plain single-goroutine FIFO over a growable ring buffer, in the zero-value-ready style of `set.Set`.
  `concurrentqueue/concurrentqueue.go` — `Queue[T]`, the concurrent twin under .NET's ConcurrentQueue vocabulary (Enqueue/TryDequeue): a
  thin facade over `concurrentlist.List[T]`, since a lock-free FIFO list and a concurrent queue are the same structure and a second
  from-scratch implementation would only be a second set of concurrency bugs to find.
- `internal/blocking/` — the bounding, blocking and completion core the three Blocking types share, over the `Store[T]` contract
  (`TryAdd`/`TryTake`/`Len`). Each park allocates its own channel on purpose: a pool would break `testing/synctest` for every caller.
  Depth for all of the above: docs/concurrency.md.
- `*/blocking.go` — `BlockingList`, `BlockingStack`, `BlockingBag`: one core, three vocabularies (Append/Take, Push/Pop, Add/Take).
  Each package re-exports the same `ErrCompleted` value and its own `WithCapacity`, so one consumer handles all three.
- `cmd/example/main.go` — a runnable tour of the packages.
- `*/bench_test.go` — the `BenchmarkCompare*` suite. Every benchmark runs the same workload on the library type AND on what a caller writes instead: a
  `map[T]struct{}` for set, a map-plus-sort and a sorted slice for sortedmap, a mutex-guarded callback slice for event. Sub-benchmarks are named
  `n=<size>/<impl>` so `benchstat` can diff them. Results go to package-level sinks, or the compiler deletes the work being measured. The event
  baseline is deliberately NOT equivalent — it holds callbacks strongly and dispatches under the lock, which is what the event's weak references and
  its pre-dispatch snapshot cost. Headline findings live in README.md.
- The plain `Benchmark*` functions at the foot of each `*_test.go` measure one implementation alone. They predate the comparison suite and are NOT
  superseded by it: the two answer different questions, and the Compare prefix exists so both keep their names. Do not delete them.
- `.github/workflows/ci.yml` — one `build` job running `wow-look-at-my/go-toolchain@v1`. The permissions block is the one go-toolchain documents;
  every entry in it guards a hard failure.

## Code Conventions

- Go module: `github.com/wow-look-at-my/go-containers`
- Test assertions: `github.com/stretchr/testify` (`assert`/`require`)
- This library is the remedy go-toolchain's `mapset` vet check names. A `map[K]struct{}`, or a `map[K]bool` whose values are all `true`, is a set
  written by hand — in this repo and in every consumer, it is `set.Set[K]`.

## Documentation

- Keep `README.md` current when a package gains or loses API. It is the human
  front page: short, with one example per package.
- This file is an index. Depth belongs in `docs/<topic>.md` with a pointer here.

## Concurrency

- Depth on every concurrent collection lives in `docs/concurrency.md`: the segment layout, the ABA argument, the blocking core, and
  the two rules that keep the concurrent benchmarks honest.
- `go-toolchain` on its own does NOT run the tests under the race detector. Run `GOFLAGS=-race go-toolchain --cgo` before you trust a
  change to any of these packages. CI has a second job that does exactly that, because green must mean race-checked here.
