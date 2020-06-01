# FaRM

1. Differency with Spanner

    1. Spanner solved replicas between different data centers, FaRM focus in one DC.
    2. Spanner is practically used, FaRM is a prototype.
    3. Spanner think the bottleneck on network, FaRM think on CPU

2. Setup
    configuration manager, using ZooKeeper, chooses primaries/backups
    sharded primary/backup replication
    can recover as long as at least one replica of each shard

3. Reasons for high performance

    1. sharding over many servers

    2. data must fit in total RAM (so no disk reads)

    3. NVRAM non-volatile RAM (so no disk writes)

        1. batteries in every rack, can run machines for a few minutes to power when main power fails
        2. Servers writes FaRM's RAM to SSD; may take a few minutes, then machine shuts down
        3. On re-start, FaRM reads saved memory image from SSD

    4. Kernel bypass

        application directly interacts with NIC

    5. one-sided RDMA (Remote DMA)

        1. remote NIC directly reads/writes memory

        2. RDMA NICs use reliable protocol

        3. Optimistic Concurrency Control

            1. Server Memory Layout

                1. regions, each an array of objects

                    object layout
                    	header with version #, and lock flag in high bit of version #

                2. for each other server
                        incoming log
                        incoming message queue

            2. Before committing, FaRM only write locally.

            3. <img src="/Users/cbc/Library/Application Support/typora-user-images/截屏2020-05-1816.31.53.png" alt="截屏2020-05-1816.31.53" style="zoom:50%;" />

            4. when primary processes COMMIT-PRIMARY in its log:
                  copy new value over object's memory
                  increment object's version #
                  clear object's lock flag

















