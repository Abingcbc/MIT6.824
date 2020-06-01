# Distributed Transactions

1. Distributed transactions = concurrency control + atomic commit
2. Correct behavior for a transaction: ACID
    1. Atomic: all or none
    2. Consistence: 
    3. Isolated: no interference between xactions -- serializable
    4. Durable: committed writes are permanent
3. The results are serializable if there exists a serial execution order of the transactions that yields the same results as the actual execution. So the right results may be multiple because of different orders.
4. Concurrency control: isolation serializability
    1. Pessimistic
        1. Two-phase locking
            1. a transaction must acquire a record's lock before using it
            2. a transaction must hold its locks until *after* commit or abort
        2. Two-phase locking may cause deadlock.
    2. Optimistic (next lecture)

5. Atomic commit

    1. Two-phase commit

        1. Data is sharded among multiple servers
            Transactions run on "transaction coordinators" (TCs)
            Server storing the data is a "participant". Each participant manages locks for its shard of the data
            TC assigns unique transaction ID (TID) to each transaction
            Every message, every table entry on the participant tagged with TID to avoid confusion

        2. Using pre-commit

            Participants must persist all the log in this transaction on disk before replying yes to pre-commit.

            They must write all the transaction before replying commit.

        3. TC can time out and abort. Therefore, other participant can release locks.

        4. Participants can unilaterally abort before replying yes. But after that, it can't unilaterally commit. After pre-committing, the commit/abort decision is made by a single entity -- the TC.

        5. TC also need to persist logs before send commits. TC can forget the transaction after they reply to the client.

        6. Participants can completely forget about a committed transaction after it acknowledges the TC's COMMIT message. 
            If it gets another COMMIT, and has no record of the transaction, it must have already committed and forgotten, and can acknowledge (again).

























