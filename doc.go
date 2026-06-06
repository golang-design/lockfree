// Copyright 2026 The golang.design Initiative authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.

// Package lockfree offers concurrent data structures organized by their
// non-blocking progress guarantee, so callers can choose the variant that fits
// their use case.
//
// The implementations live in two subpackages:
//
//   - lockfree/lf — lock-free structures: some operation always makes
//     system-wide progress with no locks, faster in the common case, callers
//     tolerate occasional retries. Stack, Queue, SkipList, OrderedMap.
//   - lockfree/wf — wait-free structures: every operation completes in a
//     bounded number of its own steps regardless of scheduling (no starvation),
//     at a higher constant cost. RingBuffer (SPSC), Queue (Kogan & Petrank).
//
// Wait-free is strictly stronger than lock-free, so a wait-free type is also
// lock-free; the wf package exists to give a bounded-latency choice where that
// matters (real-time, SLO-sensitive paths).
//
// This root package itself holds only guarantee-neutral pieces: the ADT
// contracts (Queue, Stack, Map) that both subpackages implement and that a
// single conformance suite verifies; the Less comparator; BinarySearch; and
// AddFloat64.
//
// The race detector verifies memory safety but cannot prove a progress
// guarantee; the argument for each type lives in its doc comment. Memory
// reclamation relies on Go's garbage collector, which also avoids the ABA hazard
// without hazard pointers.
//
// Note that this package is under development and not for production use.
package lockfree // import "golang.design/x/lockfree"
