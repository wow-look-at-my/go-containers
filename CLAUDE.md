# CLAUDE.md

## Build & Test

Run `go-toolchain` (no arguments) in the repository root. It tidies, vets,
tests with coverage, and builds. Never run a bare `go` command.

## Project Structure

- `set/set.go` — `Set[T comparable]`, a map with `struct{}` values behind an API. The zero value is an empty set, so every mutating method takes a
  pointer receiver and creates the map on first use; the read-only and algebraic methods take a value receiver and read a nil map fine. Union,
  Intersection, and IsDisjoint iterate the SMALLER set on purpose, and RemoveSet picks its side the same way.
- `set/json.go` — a set marshals to a JSON array of its elements and unmarshals from one, replacing whatever it held. An empty array leaves the map
  nil rather than allocating.
- `sortedmap/sortedmap.go` — `SortedMap[K, V]`, a left-leaning red-black tree. Keys stay in order at all times, so Put, Get, Delete, Min, Max, Floor
  and Ceiling are O(log n) and no walk ever sorts. `New` orders with `cmp.Compare`; `NewWithCompare` takes a comparison function for a key type with
  no natural order. The zero value is NOT usable — the comparison function is nil. Iterators: All, Keys, Values, Backward, and the half-open Range.
- `event/event.go` — `Event[T EventArgs]`, a thread-safe dispatcher whose callbacks are WEAK pointers, so an event never keeps a dead subscriber
  alive. The caller must retain its own `*func(T) error`. Invoke calls every live callback even after one fails, joins the errors, and drops the
  callbacks whose referents are gone. T must embed `event.Args`, which is what stops a bare `int` argument that could never gain a field.
- `cmd/example/main.go` — a runnable tour of the packages.
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
