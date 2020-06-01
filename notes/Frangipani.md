# Frangipani

**A network file system**

Not quite useful nowadays but its local caching is interesting

1. Structure

    users; workstations + Frangipani; network; petal

    Petal store directories, i-nodes, file content blocks, free bitmaps like an ordinary hard drive.

2. Frangipani doesn't care too much about security.

3. Cache Coherence

    1.   lock server (LS), with one lock per file/directory

        ```
            file  owner
            -----------
            x     WS1
            y     WS1
            
          workstation (WS) Frangipani cache:
            file/dir  lock  content
            -----------------------
            x         busy  ...
            y         idle  ...
        ```

        if WS holds lock,
            busy: using data right now
            idle: holds lock but not using the cached data right now. Hold recent modified file's lock is advantage

    2. workstation rules:
        1. don't cache unless you hold the lock
        2. acquire lock, then read from Petal
        3. write to Petal, then release lock

    3. coherence protocol messages:
        1. request  (WS -> LS)
        2. grant (LS -> WS)
        3. revoke (LS -> WS) ask idle lock to release
        4. release (WS -> LS)
    4. before a workstation releases a lock on a file, it must write modified file content to Petal, as well as meta-data
    5. Frangipani has shared read locks, as well as exclusive write locks. One WS can write only when every Read release their lock.

4. Atomicity

    WS acquires locks on all file system data that it will modify. WS performs operation with all locks held and only releases all when all the operation are finished.

5. Crash Recovery

    1. WAL, but Frangipani's doesn't like traditional ones

        1. Frangipani has a separate log for each workstation. This avoids a logging bottleneck, eases decentralization but scatters updates to a given file over many logs
        2. Frangipani's logs are in shared Petal storage, not local disk. WS2 can read WS1's log to recover from WS1 crashing

    2. Log Entry

        ```
        log sequence number
        array of updates:
        	block #, new version #, addr, new bytes
        ```

        just contains meta-data updates, not file content updates

        initially the log entry is in WS local memory (not yet Petal) and avoid to send log to petal as long as possible

    3. What happened when WS receives a revoke message

        1. Write log to Petal
        2. Write modified blocks to Petal
        3. Send release

    4. WS1 crashes when holding a lock

        Nothing will happen until WS2 requests a lock that WS1 holds
        LS sends revoke to WS1, gets no response and when LS times out, it will tell WS2 to recover WS1 from its log in Petal

        WS3 will read WS1's log from Petal, performs Petal writes described by logged operations and tell LS it is done, so LS can release WS1's locks

    5. Version number is in each meta-data block (i-node) in Petal. Version number(s) in each logged op is block's version plus one

        Recovery replays only if op's version > block version.

    6. The consequences of log not containing data

        Use fsync()

6. WS cannot write with lock not in its lease (when partitioned and then recover)

















