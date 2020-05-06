# Lab3 Raft-based KV

get也需要提交

Select default 只由leader发不行，都发阻塞applyLoop



Logentry 必须要有index了

applyMsg用来给follower同步storage

> Raft also includes a small amount of metadata
>  in the snapshot: the last included index is the index of the
>  last entry in the log that the snapshot replaces (the last entry the state machine had applied), and the last included
>  term is the term of this entry. These are preserved to support the AppendEntries consistency check for the first log
>  entry following the snapshot, since that entry needs a previous log index and term.

所以我这边不能把最后的抹成空的term，需要把最后一个term也记进去。不然leader之后发送pre log term 永远是0，然后follower就会一直reply false。因为//1. Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm (§5.3)

