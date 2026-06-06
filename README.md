# lockfree

[![PkgGoDev](https://pkg.go.dev/badge/golang.design/x/lockfree)](https://pkg.go.dev/golang.design/x/lockfree) [![Go Report Card](https://goreportcard.com/badge/golang.design/x/lockfree)](https://goreportcard.com/report/golang.design/x/lockfree)
![lockfree](https://github.com/golang-design/lockfree/workflows/lockfree/badge.svg?branch=master)


Package lockfree offers concurrent data structures in Go (generics, Go 1.26+),
organized by their non-blocking progress guarantee so you can pick the variant
that fits your use case.

```
import (
	"golang.design/x/lockfree/lf" // lock-free
	"golang.design/x/lockfree/wf" // wait-free
)
```

Each structure documents its *precise* guarantee rather than a blanket
"lock-free" claim, because they differ:

| Package | Type | Guarantee | Algorithm |
|---------|------|-----------|-----------|
| `lf` | `Stack[T]` | lock-free | Treiber |
| `lf` | `EliminationStack[T]` | lock-free | Hendler, Shavit & Yerushalmi (elimination backoff) |
| `lf` | `Queue[T]` | lock-free | Michael & Scott |
| `lf` | `Deque[T]` | lock-free | Sundell & Tsigas (doubly linked, single-word CAS) |
| `lf` | `SkipList[K,V]` | lock-free (`Get`/`Search` wait-free) | Herlihy & Shavit, marked pointers |
| `lf` | `OrderedMap[K,V]` | lock-free | backed by `SkipList` |
| `lf` | `List[K,V]` | lock-free | Harris & Michael ordered list |
| `lf` | `HashMap[K,V]` | lock-free | Michael (bucketed lists, fixed size) |
| `lf` | `SplitHashMap[K,V]` | lock-free | Shalev & Shavit split-ordered (resizable) |
| `wf` | `Queue[T]` | wait-free | Kogan & Petrank |
| `wf` | `Stack[T]` | wait-free | Herlihy universal construction |
| `wf` | `RingBuffer[T]` | wait-free, **bounded SPSC** (one producer, one consumer) | array + cursors |
| (root) | `AddFloat64` | lock-free | atomic CAS loop |

*Wait-free* means every operation finishes in a bounded number of its own steps
(no starvation); *lock-free* means the system as a whole always makes progress
with no locks and no operation waiting on another. Wait-free is strictly stronger
than lock-free, so the `wf` variants trade a higher constant factor for a
bounded-latency guarantee.

### Choosing a guarantee

```go
s, _ := lf.NewQueue[int](), 0 // lock-free: unbounded callers, no ceremony
s.Enqueue(1)

q := wf.NewQueue[int](maxGoroutines) // wait-free: bounded participants
h := q.Handle()                      // one Handle per goroutine
h.Enqueue(1)
```

The wait-free `Queue` and `Stack` need participants to register, because their
helping mechanism is indexed by a participant id and Go has no goroutine id.
Acquire one handle per goroutine, up to `maxHandles`; the slots are not
reclaimable, so this fits a bounded, long-lived worker pool. Each operation is
O(`maxHandles`). Both the lock-free and wait-free queues satisfy the shared
`lockfree.Queue[T]` interface (and likewise for stacks), but the swap is not
symmetric: on the `wf` side it is the per-goroutine handle, not the `Queue` or
`Stack` value, that satisfies the interface.

### Performance

The benchmarks (`go test -bench`) sweep each structure across goroutine counts
under a balanced, prefilled workload, comparing every implementation of an ADT.
The qualitative conclusions, which hold across machines even though the exact
numbers do not:

- **Stacks and queues:** lock-free roughly matches a mutex at low concurrency and
  both degrade as goroutines oversubscribe the cores. The **elimination stack is
  the exception**: it stays flat while goroutines fit on cores, because colliding
  pushes and pops cancel out instead of contending on the top pointer.
- **Deque:** the lock-free Sundell & Tsigas deque is *slower* than a
  `container/list` behind a mutex on raw throughput; its helping protocol costs
  more than a lock here. Choose it for the non-blocking progress guarantee, not
  for speed.
- **Maps:** the lock-free hash maps scale on reads while mutex maps serialize.
  An `RWMutex` map helps reads but not writes; the lock-free maps beat it on
  both. The skip list has a high per-operation constant (ordered, pointer
  chasing) but scales with cores without a contention bottleneck.
- **Wait-free** types are slower on every throughput number because each
  operation scans participant state and helps stalled peers. That is the trade:
  they buy a bounded worst-case latency per operation that an averaged throughput
  figure cannot show. Reach for them on latency-sensitive paths, not for peak
  throughput.

Illustrative numbers (ns/op, lower is better) on an Apple M2 (8 cores), Go
1.26.1. The `g=1` column is single-threaded latency; `g=8`/`g=32` are per-op time
under parallelism (so a value that *drops* from `g=1` to `g=8` is scaling with
cores). Reproduce with `go test -run '^$' -bench=. -benchmem ./...`.

| ns/op | g=1 | g=8 | g=32 |
|-------|----:|----:|-----:|
| Stack mutex | 12 | 101 | 102 |
| Stack lock-free (Treiber) | 19 | 153 | 118 |
| Stack lock-free (elimination) | 21 | 21 | 127 |
| Stack wait-free | 168 | 396 | 400 |
| Queue mutex | 15 | 105 | 109 |
| Queue lock-free | 17 | 178 | 175 |
| Queue wait-free | 122 | 379 | 381 |
| Deque mutex (`container/list`) | 40 | 123 | 125 |
| Deque lock-free | 147 | 160 | 190 |

Maps, read-heavy (~7/8 Get) and write-heavy (1/2 Get, 1/4 Set, 1/4 Del):

| ns/op | read g=8 | read g=32 | write g=8 | write g=32 |
|-------|---------:|----------:|----------:|-----------:|
| mutex map | 120 | 138 | 140 | 150 |
| RWMutex map | 60 | 64 | 127 | 81 |
| `sync.Map` | 10 | 10 | 25 | 20 |
| lock-free skip list | 101 | 118 | 285 | 457 |
| lock-free hash map (sized) | 9 | 10 | 27 | 26 |
| lock-free split hash map | 16 | 43 | 31 | 37 |

### Verification

The race detector checks memory safety but cannot prove a progress guarantee, so
the argument for each lives in its doc comment. Behavior is verified by a shared
conformance suite (`internal/conformtest`) run against every implementation of an
ADT, plus differential fuzzing and oversubscribed contention tests under `-race`.

`RingBuffer` is single-producer/single-consumer only; using multiple producers or
consumers will corrupt it. `Len` and `Range` on the maps are weakly consistent
under concurrency.

## Contributing

We would love to have your experiences. Feel free to [submit an issue](https://golang.design/x/lockfree/issues/new) for requesting a new implementation or bug report.

## License

MIT &copy; [Changkun Ou](https://changkun.de)
