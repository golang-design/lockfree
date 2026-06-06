# Changelog

All notable changes to this project are documented here.

## v0.1.0

First release of the rewritten package. This is a near-complete overhaul of the
2019 `v0.0.1` snapshot, which shipped only a root-package queue, stack, and
`AddFloat64`. Every data structure is now generic (Go 1.26+), documents its
*precise* non-blocking progress guarantee, and is grounded in a cited,
peer-reviewed source (see [REFERENCES.md](REFERENCES.md)).

### Breaking changes

- **Reorganized by progress guarantee into two subpackages.** Lock-free
  structures live in `golang.design/x/lockfree/lf`; wait-free structures live in
  `golang.design/x/lockfree/wf`. The old root-package concrete types are removed
  (not aliased). Update imports: `lockfree.NewQueue` becomes `lf.NewQueue`, and so
  on. The root package now holds only shared, guarantee-neutral helpers
  (`Less`, `BinarySearch`, `AddFloat64`) and the shared ADT interfaces.
- **Requires Go 1.26+.** All structures are generic.

### Added: lock-free structures (`lf`)

- `Stack[T]` (Treiber).
- `EliminationStack[T]` (Hendler, Shavit & Yerushalmi elimination backoff).
- `Queue[T]` (Michael & Scott).
- `Deque[T]` (Sundell & Tsigas, doubly linked, single-word CAS).
- `SkipList[K,V]` and `OrderedMap[K,V]` (Herlihy & Shavit, marked pointers;
  `Get`/`Search` are wait-free).
- `List[K,V]` (Harris / Michael ordered list-based map).
- `HashMap[K,V]` (Michael, fixed bucket count).
- `SplitHashMap[K,V]` (Shalev & Shavit split-ordered, resizable).

### Added: wait-free structures (`wf`)

- `Queue[T]` (Kogan & Petrank; handle-per-participant model).
- `Stack[T]` (Herlihy universal construction).
- `Deque[T]` (Herlihy universal construction over a persistent AVL tree;
  O(maxHandles * log n), size-dependent).
- `Map[K,V]` (Herlihy universal construction over a persistent AVL search tree;
  O(maxHandles * log n), reads linearized too).
- `RingBuffer[T]` (bounded single-producer/single-consumer).

### Added: quality and tooling

- A shared conformance suite (`internal/conformtest`) runs identical behavioral
  tests across every implementation of an ADT.
- Contention-swept benchmarks comparing mutex vs lock-free vs wait-free across
  goroutine counts.
- Differential and fuzz tests for the non-trivial structures.
- [REFERENCES.md](REFERENCES.md): full academic citations for every implemented
  algorithm, each verified against its primary source.
- CI runs build, `-race`, fuzz, and benchmark jobs.

### Deferred

- **Lock-free bounded MPMC ring (Tsigas & Zhang, SPAA 2001).** Attempted, but the
  port is not yet faithful: it passes single-threaded and single-producer tests
  while reordering and stranding values under concurrent producers. Rather than
  ship an unverified guarantee, it is held back pending a correct port. See
  [TODO.md](TODO.md) for the failure analysis and the path forward.
