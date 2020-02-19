# Introduction

* What is a distributed system?
  * multiple cooperating computers
  * storage for big web sites, MapReduce, peer-to-peer sharing, &c
  * lots of critical infrastructure is distributed

* Why do people build distributed systems?
  *  to increase capacity via parallelism
  * to tolerate faults via replication
  * to place computing physically close to external entities
  * to achieve security via isolation
* Challenge of distributed system
  * many concurrent parts, complex interactions
  * must cope with partial failure
  * tricky to realize performance potential
* Distrubited System Performance
  * Scalability: not infinite because somewhere else may become a bottleneck again
* Faut tolerance
  * Availbility: Only on certain set of failures
  * Recoverability
    * Non-volite storage
    * Replication
* Consistency
  * Strong: too expensive
  * Week



## MapReduce

* MapReduce limits what apps can do:
  * No interaction or state (other than via intermediate output).
  * No iteration, no multi-stage pipelines.
  * No real-time or streaming processing.
* What does MR send over the network?
  * Maps read input from GFS.
  * Reduces read Map output.