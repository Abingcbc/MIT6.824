# Chain Replication CRAQ

<img src="img/截屏2020-05-0217.24.59.png" alt="截屏2020-05-0217.24.59" style="zoom:50%;" />

1. It seems to be used in HDFS writing.
2. Why is CR attractive (vs Raft)?
   1. Client RPCs split between head and tail, vs Raft's leader handles both.
   2. Head sends each write just once, vs Raft's leader sends to all.
   3. Reads involve just one server, not all as in Raft.
   4. Situation after failure simpler than in Raft 
3. It cannot work alone, some authority (configuration manager) is needed to decide server's dead
4. 

