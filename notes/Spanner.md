# Spanner

1. Replications are divided into shards and one Paxos group per shard.

2. Spanner treats read/write and read/only transactions differently.

    1. Read/write transaction

        Two-phase commit

        Choose a Paxos group to act as Transaction Coordinator

        1.   Transaction Coordinator:
            1. Decides commit or abort.
            2. Logs the decision to its group via Paxos.
            3. Tell participant leaders and client the result.
        2. Each participant leader:
            1. Log the TC's decision via Paxos.
            2. Release the transaction's locks.

    2. Read-only transaction

        1. Correctness constraints

            1. Serializable
            2. Externally consistent

        2. Snapshot isolation

            Synchronize all computers' clocks (to real wall-clock time).
            Assign every transaction a time-stamp.
            	r/w: commit time.
            	r/o: start time.
            Execute as if one-at-a-time in time-stamp order.
            	Even if actual reads occur in different order.
            Each replica stores multiple time-stamped versions of each record.
            	All of a r/w transactions's writes get the same time-stamp.
            	An r/o transaction's reads see version as of xaction's time-stamp and read the record 	version with the highest time-stamp less than the xaction's.

        3. Safe time

            Paxos leaders send writes in timestamp order. Replicas record the latest timestamp received from leader. 

            Replicas clients read from must assume that record must be as latest as leader's. Or they will delay.

        4. Clock Sync

            1. Not precisely

                1. Too large

                    Correct but slow

                2. Too small

                    May miss some transactions

            2. TrueTime

                1. Time service yields a TTinterval = [ earliest, latest ].
                2. The correct time is guaranteed to be somewhere in the interval.
                3. Start rule:
                    xaction TS = TT.now().latest
                    for r/o, at start time
                    for r/w, when commit begins
                4. Commit wait, for r/w xaction:
                    Before commit, delay until TS < TS.now().earliest
                    Guarantees that TS has passed.