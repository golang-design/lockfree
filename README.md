# lockfree

[![PkgGoDev](https://pkg.go.dev/badge/golang.design/x/lockfree)](https://pkg.go.dev/golang.design/x/lockfree) [![Go Report Card](https://goreportcard.com/badge/golang.design/x/lockfree)](https://goreportcard.com/report/golang.design/x/lockfree)
![lockfree](https://github.com/golang-design/lockfree/workflows/lockfree/badge.svg?branch=master)


Package lockfree offers concurrent data structures with non-blocking progress
guarantees in Go (generics, Go 1.26+).

```
import "golang.design/x/lockfree"
```

Each structure documents its *precise* progress guarantee rather than a blanket
"lock-free" claim, because they differ:

| Type | Guarantee | Algorithm |
|------|-----------|-----------|
| `Stack[T]` | lock-free | Treiber |
| `Queue[T]` | lock-free | Michael & Scott |
| `RingBuffer[T]` | wait-free, **bounded SPSC** (one producer, one consumer) | array + cursors |
| `SkipList[K,V]` | lock-free (`Get`/`Search` wait-free) | Herlihy & Shavit, marked pointers |
| `OrderedMap[K,V]` | lock-free | backed by `SkipList` |
| `AddFloat64` | lock-free | atomic CAS loop |

*Wait-free* means every operation finishes in a bounded number of its own steps;
*lock-free* means the system as a whole always makes progress with no locks and
no operation waiting on another. The race detector checks memory safety but
cannot prove these guarantees; the argument for each is in its doc comment, and
correctness is exercised by conservation/differential/contention tests under
`-race`.

`RingBuffer` is single-producer/single-consumer only; using multiple producers
or consumers will corrupt it. `Len` and `Range` on the maps are weakly
consistent under concurrency.

## Contributing

We would love to have your experiences. Feel free to [submit an issue](https://golang.design/x/lockfree/issues/new) for requesting a new implementation or bug report.

## License

MIT &copy; [Changkun Ou](https://changkun.de)
