# Amazon Aurora

1. DB server modifies only cached data pages as transaction runs and appends update info to WAL (Write-Ahead Log).

2. WAL contains both the old value and the new value.

3. Big picture

    1. DB clients
    2. One database server on an EC2 instance
    3. Six storage servers store six replicas of a volume. Each volume is private to the one database server.

4. Aurora only replicate log entries and only wait for any four replicas to commit.

5. Fault-tolerant goals for storage server

    1. be able to write even if one AZ entirely dead

    2. be able to read even if one dead AZ and one more storage server dead

        no writes in this case but can still serve r/o requests, and create more replicas

    3. tolerate temporarily slow replicas smoothly (a big deal)

    4. repair dead replica very quickly (in case another failure!)

6. Quorum Read/Write technique

    N replicas

    Write to any W replicas

    Read from any R replicas

    R + W >= N + 1

    writer assigns increasing version numbers to its writes, reader takes max version # of the R copies it receives

7. The benefit of Quorum Read/Write storage system

    Smooth handling of dead or slow or partitioned storage servers because of only waiting for the fastest W/R replicas

8. Raft use quorum too

    Aurora uses N=6, W=4, R=3

9. In my opinion, quorum is an application of **pigeonhole principle**.

10. Aurora's write doesn't modify existing data items. It only appends a Write log or a Commit log. Transaction commit waits for all of xaction's log records to be on a quorum.

11. Storage servers only store an old version of each data page and the list of log records required to bring the page up to date.

    Storage server applies log records in the background or when needed by a read.

12. Aurora tracks the highest number of log entries in each storage server. So in ordinary situation, DB server only needs read from the latest storage server.

    Aurora only uses quorum read during crash recovery. Aurora finds the highest missing log number and asks all the Storage Servers to delete all the after it.

13. For too big databases, data pages are sharded into 10-GB Protection Groups. Each PG is separately replicated as six "segments" (on six replicas). Different PGs likely to be on different sets of six storage servers.

14. DB server sends each PG just the log records relevant to that PG's data pages.

15. When one server fails, each PG can re-replicate fully in parallel:

     Each PG can be sent from a different source server, to a different dst.

16. Read-only database server

    1. Results may be out-of-date.
    2. Main DB server sends logs to Read-only DB.
    3. Read-only DB server should ignore uncommitted logs.
    4. When storage servers are rebalancing B-tree, they should avoid read-only DB to read.

17. Aurora considers network a worse constraint than CPU or storage.















