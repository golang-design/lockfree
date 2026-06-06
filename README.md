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
