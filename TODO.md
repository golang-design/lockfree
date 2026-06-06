# TODO LIST

## Status

Implemented and genuinely non-blocking (see each type's doc comment for its
precise progress guarantee and the tests that exercise it under `-race`). They
are organized by guarantee into the `lf` (lock-free) and `wf` (wait-free)
subpackages, and a shared conformance suite verifies every implementation of an
ADT behaves identically. Full academic citations for each implemented algorithm
are in [REFERENCES.md](REFERENCES.md):

- `lf.Stack[T]`: lock-free (Treiber)
- `lf.EliminationStack[T]`: lock-free (Hendler, Shavit & Yerushalmi elimination)
- `lf.Queue[T]`: lock-free (Michael & Scott)
- `lf.Deque[T]`: lock-free (Sundell & Tsigas, doubly linked, single-word CAS)
- `lf.SkipList[K,V]` / `lf.OrderedMap[K,V]`: lock-free (Herlihy & Shavit, marked
  pointers); this also covers the "provably correct scalable concurrent skip
  list" line below in textbook lock-free form
- `lf.List[K,V]`: lock-free ordered list-based map (Harris / Michael)
- `lf.HashMap[K,V]`: lock-free hash table (Michael, fixed bucket count)
- `lf.SplitHashMap[K,V]`: lock-free resizable hash table (Shalev & Shavit split-ordered lists)
- `wf.Queue[T]`: wait-free (Kogan & Petrank)
- `wf.Stack[T]`: wait-free (Herlihy's universal construction)
- `wf.Deque[T]`: wait-free (Herlihy's universal construction over a persistent
  AVL tree; O(maxHandles * log n), size-dependent unlike Queue/Stack)
- `wf.Map[K,V]`: wait-free ordered map (Herlihy's universal construction over a
  persistent AVL search tree; O(maxHandles * log n), reads linearized too)
- `wf.RingBuffer[T]`: wait-free bounded SPSC
- `AddFloat64`: lock-free

The papers below remain open. They fall into three groups, none of which adds a
non-blocking guarantee or ADT the package does not already provide:

- **Alternative variants of a shipped ADT (low marginal value).** Valois's
  lock-free list, the Shann/Fober/Evequoz queues, and the Fomitchev-Ruppert
  skip list are different constructions of structures already shipped (lock-free
  list, queue, and skip list). They would add implementation variety, not a new
  capability.
- **Bounded MPMC ring (Tsigas-Zhang): attempted, deferred.** The one genuine
  capability gap, a lock-free bounded MPMC FIFO. The attempt is not faithful yet
  (see the Queue section below); shipping it would violate the project's rule
  that every claimed guarantee be verified, so it is held back rather than
  shipped buggy.
- **Research-grade concurrent trees (high effort).** The Bronson BST,
  Braginsky-Petrank B+ tree, and Kim lock-free red-black tree are large,
  subtle algorithms. The package already offers ordered, logarithmic structures
  (`lf.SkipList`/`lf.OrderedMap` lock-free, `wf.Map` wait-free), so these are
  deferred as future depth, not a coverage gap.

## List of Algorithms

### Linked List

- [x] Harris, Timothy L. "A pragmatic implementation of non-blocking linked-lists." International Symposium on Distributed Computing. Springer, Berlin, Heidelberg, 2001. [PDF](https://pdfs.semanticscholar.org/68a9/005a5ec10daece36ca5ecb9cad7be44770b1.pdf)
- [x] Sundell, Hakan, and Philippas Tsigas. "Lock-Free and Practical Deques and Doubly Linked Lists using Single-Word Compare-And-Swap." 2004 (Implemented as `lf.Deque[T]`.) [PDF](https://pdfs.semanticscholar.org/8a68/f45bd32ed050a96faa24139ab71178258f13.pdf)
- [ ] Valois, John D. "Lock-free linked lists using compare-and-swap." PODC. Vol. 95. 1995. [PDF](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.41.9506&rep=rep1&type=pdf)

### Queue

- [x] Maged M. Michael and Michael L. Scott. 1996. Simple, fast, and practical non-blocking and blocking concurrent queue algorithms. In Proceedings of the fifteenth annual ACM symposium on Principles of distributed computing (PODC '96). ACM, New York, NY, USA, 267-275. [PDF](https://apps.dtic.mil/dtic/tr/fulltext/u2/a309412.pdf)
- [ ] Shann, Chien-Hua, Ting-Lu Huang, and Cheng Chen. "A practical nonblocking queue algorithm using compare-and-swap." Proceedings Seventh International Conference on Parallel and Distributed Systems (Cat. No. PR00568). IEEE, 2000. [PDF](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.199.7928&rep=rep1&type=pdf)
- [ ] Fober, Dominique, Yann Orlarey, and Stéphane Letz. "Optimised lock-free FIFO queue." (2001). [PDF](https://hal.archives-ouvertes.fr/hal-02158792/document)
- [ ] Evequoz, Claude. "Non-blocking concurrent fifo queues with single word synchronization primitives." 2008 37th International Conference on Parallel Processing. IEEE, 2008. [PDF](https://www.liblfds.org/downloads/white%20papers/%5BQueue%5D%20-%20%5BEvequoz%5D%20-%20Non-Blocking%20Concurrent%20FIFO%20Queues%20With%20Single%20Word%20Synchroniation%20Primitives.pdf)
- [ ] Tsigas, Philippas, and Yi Zhang. "A simple, fast and scalable non-blocking concurrent FIFO queue for shared memory multiprocessor systems." Proceedings of the thirteenth annual ACM symposium on Parallel algorithms and architectures (SPAA '01). 2001. **Deferred: attempted as a lock-free bounded MPMC ring (`lf.RingBuffer`).** The port's control flow matches Figures 6 and 7 and passes single-threaded and single-producer tests, but under concurrent producers it both reorders a producer's own values (a later `Put` fills a transient hole behind the real tail) and strands values (a `Put` lands on or behind the head gap cell), failing conservation and per-producer FIFO. Four permutations of the core mechanism (m = 1/2 x restore-same/restore-opposite) all failed, which indicates the lag invariant is not being maintained rather than a single typo. The likely gap is the two-NULL re-encoding: the paper appears to select the empty marker positionally at dequeue time, whereas the attempt baked the choice into each entry at enqueue time. The published algorithm is sound; deferred pending a verified, faithful port. The abandoned attempt lives on the `lf-mpmc-ring` branch, not on master. [PDF](https://www.cse.chalmers.se/~tsigas/papers/latest-spaa01.pdf)

### Skip-list

- [ ] Fomitchev, Mikhail, and Eric Ruppert. "Lock-free linked lists and skip lists." Proceedings of the twenty-third annual ACM symposium on Principles of distributed computing. ACM, 2004. [PDF](http://people.scs.carleton.ca/~edwardduong/PDF_files_of_relevant_papers/2004%20-%20Lock-free%20Linked%20List%20and%20Skip%20Lists.pdf)
- [x] Herlihy, Maurice, et al. "A provably correct scalable concurrent skip list." Conference On Principles of Distributed Systems (OPODIS). 2006. (Implemented as `SkipList[K,V]` in the textbook lock-free, marked-pointer form.) [PDF](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.170.719&rep=rep1&type=pdf)

### Stack

- [x] Hendler, Danny, Nir Shavit, and Lena Yerushalmi. "A scalable lock-free stack algorithm." Proceedings of the sixteenth annual ACM symposium on Parallelism in algorithms and architectures. ACM, 2004. [PDF](http://www.inf.ufsc.br/~dovicchi/pos-ed/pos/artigos/p206-hendler.pdf)

### Tree

- [ ] Bronson, Nathan G., et al. "A practical concurrent binary search tree." ACM Sigplan Notices. Vol. 45. No. 5. ACM, 2010. [PDF](http://www.academia.edu/download/42135309/ppopp207-bronson.pdf)
- [ ] Braginsky, Anastasia, and Erez Petrank. "A lock-free B+ tree." Proceedings of the twenty-fourth annual ACM symposium on Parallelism in algorithms and architectures. ACM, 2012. [PDF](http://www.cs.technion.ac.il/~erez/Papers/lfbtree-full.pdf)
- [ ] Kim, Jong Ho, Helen Cameron, and Peter Graham. "Lock-free red-black trees using cas." Concurrency and Computation: Practice and experience (2006): 1-40. [PDF](https://www.cs.umanitoba.ca/~hacamero/Research/RBTreesKim.pdf)

### Hash

- [x] Michael, Maged M. "High performance dynamic lock-free hash tables and list-based sets." Proceedings of the fourteenth annual ACM symposium on Parallel algorithms and architectures. ACM, 2002. [PDF](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.114.5854&rep=rep1&type=pdf)