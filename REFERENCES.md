# References

Every concurrent algorithm in this module is a port of a published, peer-reviewed
result. This file maps each implemented type to its source so the literature is
easy to find; each type's doc comment carries the same citation and the
implementation notes. Forward-looking candidates not yet implemented live in
[TODO.md](TODO.md).

## `lf` — lock-free

- **`lf.Stack[T]`** — Treiber stack.
  R. K. Treiber. "Systems Programming: Coping with Parallelism." IBM Almaden
  Research Center, Technical Report RJ 5118, 1986.

- **`lf.EliminationStack[T]`** — elimination-backoff stack.
  Danny Hendler, Nir Shavit, and Lena Yerushalmi. "A scalable lock-free stack
  algorithm." SPAA 2004.
  [PDF](http://www.inf.ufsc.br/~dovicchi/pos-ed/pos/artigos/p206-hendler.pdf)

- **`lf.Queue[T]`** — Michael & Scott queue.
  Maged M. Michael and Michael L. Scott. "Simple, fast, and practical
  non-blocking and blocking concurrent queue algorithms." PODC 1996, 267-275.
  [PDF](https://apps.dtic.mil/dtic/tr/fulltext/u2/a309412.pdf)

- **`lf.Deque[T]`** — doubly linked list-based deque, single-word CAS.
  Hakan Sundell and Philippas Tsigas. "Lock-Free and Practical Deques and Doubly
  Linked Lists using Single-Word Compare-And-Swap." OPODIS 2004.
  [PDF](https://pdfs.semanticscholar.org/8a68/f45bd32ed050a96faa24139ab71178258f13.pdf)

- **`lf.SkipList[K,V]`** and **`lf.OrderedMap[K,V]`** — lock-free skip list with
  Harris-style marked pointers (OrderedMap is a thin facade over SkipList).
  Maurice Herlihy, Yossi Lev, Victor Luchangco, and Nir Shavit. "A provably
  correct scalable concurrent skip list." OPODIS 2006. The textbook presentation
  is in Herlihy & Shavit, "The Art of Multiprocessor Programming."
  [PDF](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.170.719&rep=rep1&type=pdf)

- **`lf.List[K,V]`** — ordered list-based set/map (the building block under the
  hash maps).
  Timothy L. Harris. "A pragmatic implementation of non-blocking linked-lists."
  DISC 2001
  ([PDF](https://pdfs.semanticscholar.org/68a9/005a5ec10daece36ca5ecb9cad7be44770b1.pdf)),
  with the deletion-marking refinement of Maged M. Michael (below).

- **`lf.HashMap[K,V]`** — lock-free hash table with a fixed bucket count.
  Maged M. Michael. "High performance dynamic lock-free hash tables and
  list-based sets." SPAA 2002.
  [PDF](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.114.5854&rep=rep1&type=pdf)

- **`lf.SplitHashMap[K,V]`** — lock-free resizable hash table via recursive
  split-ordering.
  Ori Shalev and Nir Shavit. "Split-ordered lists: Lock-free extensible hash
  tables." Journal of the ACM 53(3), 2006, 379-405.
  [DOI](https://dl.acm.org/doi/10.1145/1147954.1147958) ·
  [PDF](https://www.cs.tau.ac.il/~afek/SplitOrderListHashSS03.pdf)

## `wf` — wait-free

- **`wf.Queue[T]`** — wait-free queue with multiple enqueuers and dequeuers
  (announce array plus a priority-based helping scheme).
  Alex Kogan and Erez Petrank. "Wait-free queues with multiple enqueuers and
  dequeuers." PPoPP 2011.
  [PDF](https://dl.acm.org/doi/pdf/10.1145/1941553.1941585)

- **`wf.Stack[T]`** — wait-free stack via Herlihy's universal construction (see
  the foundational entry below).

- **`wf.Deque[T]`** — wait-free deque via Herlihy's universal construction (see
  below) over a persistent, height-balanced (AVL) tree as the immutable
  sequential state (Okasaki, see below). It is O(maxHandles · log n), so
  size-dependent, unlike the O(maxHandles) queue and stack; that log-of-size
  dependence is consistent with the dedicated state of the art:
  Shalom M. Asbell and Eric Ruppert. "A Wait-Free Deque With Polylogarithmic Step
  Complexity." OPODIS 2023.
  [DOI](https://drops.dagstuhl.de/entities/document/10.4230/LIPIcs.OPODIS.2023.17)

- **`wf.RingBuffer[T]`** — bounded single-producer/single-consumer FIFO buffer.
  Leslie Lamport. "Proving the Correctness of Multiprocess Programs." IEEE
  Transactions on Software Engineering SE-3(2), 1977; and "Concurrent Reading and
  Writing." Communications of the ACM 20(11), 1977, 806-811.

## Foundational results

- **Herlihy's wait-free universal construction** — the basis of `wf.Stack` and
  `wf.Deque`: any sequential object can be made wait-free by linearizing
  operations through consensus with round-robin helping.
  Maurice Herlihy. "Wait-free synchronization." ACM Transactions on Programming
  Languages and Systems 13(1), 1991, 124-149.
  [DOI](https://dl.acm.org/doi/10.1145/114005.102808)
  Textbook form: Herlihy & Shavit, "The Art of Multiprocessor Programming,"
  section 6.4.

- **Compare-and-swap as a universal primitive** — underpins the lock-free
  read-modify-write loop in **`AddFloat64`** (root package). Same Herlihy 1991
  paper as above.

- **Persistent (purely functional) data structures** — `wf.Deque` represents its
  sequential state as an immutable balanced tree so the universal construction
  can apply operations as pure functions.
  Chris Okasaki. "Purely Functional Data Structures." Cambridge University Press,
  1998.

## Sequential utilities

- **`BinarySearch`** (root package) is the classic binary search over a sorted
  slice; it is sequential, not a concurrency algorithm. Donald E. Knuth, "The Art
  of Computer Programming, Volume 3: Sorting and Searching," section 6.2.1.
