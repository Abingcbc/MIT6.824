# Infrastructure: RPC and threads

## Why Go?

* good support for threads
* convenient RPC
* type safe

* garbage-collected (no use after freeing problems)

* threads + GC is particularly attractive!

* relatively simple

## Thread

The threads share memory, each thread includes some per-thread state: program counter, registers, stack.

### Why thread?

* I/O concurrency
* Parallelism
* Convenience

### Event-driven and threads

 Event-driven gets you I/O concurrency, and eliminates thread costs (which can be substantial),  but doesn't get multi-core speedup, and is painful to program. But when the server needs to serve one million requests, the cost of start threads may be huge and event-driven may be better.

### Thread Challenges

* shared data
* coordination between threads
* deadlock

