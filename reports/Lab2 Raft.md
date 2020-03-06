# Lab2 Raft

这次实验实现了 Raft 算法中的 leader election 和 log replication，比上一次的 lab 难度明显大了很多。Raft 论文中忽略很多实现细节，实现的时候也遇到了很多坑。整个实现也可以分为两个部分，接下来从 Leader（Candidate） 和 Follower 这两个角度进行总结。

## Leader Election

### Candidate

在一个集群的开始，所有的 server 由于 term 都相同，所以都会在第一轮选举中投给自己，但每个 server 都会等待一个随机的时间。而正是这个随机时间，就可以使某一个 server 率先进入下一个 term 的选举。此时，他的 term 是最大的，所以可以获取到大多数的投票。这一点类似于计算机网络中 802.3 的随机后退。

这里我遇到了一个比较典型的 bug，也是自己对 Figure 2 没有完全理解。

1. 刚开始我的实现是在开启多个协程发送投票请求后，使用 WaitGroup 等待所有请求返回后再计算是否投同意票的是否大于投反对票的。但实际上，Raft 中的 majority 不是指同意票大于反对票，而是指所有 server 的多数，即使有些 server 已经 crash 掉了。

   另外，不能等待所有请求返回再计算。因为有些请求可能由于接收方已经离线了，导致超时。而超时的时间远大于选举超时的时间，这就会导致性能下降非常明显。所以，一种方法是每次收到回复后，都判断一次是否收到多数投票，我在复制 log 时采用了这种方法。在选举这里，我采用了另一种方法，就是等待一段时间（大于正常请求的通信时间），如果能够收到多数投票，那么就成为 leader。如果失败，那么进入下一轮或者转为 follower。但这种方法也存在着问题，不如前一种高效，之后重构的时候可以进行改进。

### Follower

Follower 在收到投票请求后，并不能单纯的因为请求的 term 大于自己，就投给它。我们可以考虑这样一种情况：server1 作为 follower 离线了，但由于它长时间没有收到 leader 的心跳包，它会进入新一轮的选举。但由于它离线了，不可能获取到大多数的投票，因此它仍然处于 follower 状态。然后仍然心跳包超时，进入新一轮选举。如此往复，follower 的 term 可能会变的非常大。但当它重新上线时，它的 log 显然不可能比在线的 leader 更新，所以不能获取投票。这时，投票就需要比较两者的 log 哪个更加 up-to-date。首先，比较两者最后一个 log 的 term，如果请求的 term 大，则可以投给它。反之，则不可以。如果两者 term 相同，则比较 index。index 大的更新。

当然，如果请求的 term 小于自身，不会投票给它。



## Log Replication

### Leader

leader 在接收到 log 后，append 到自己的数组中。接下来，向 follower 广播 log。对于每个 follower，为了保证数据的一致，leader 需要维护一个 nextIndex 数组，存放每个 server 接下来需要的 log 的 index。由于 nextIndex 初始化为 leader 的 lastApplied + 1，所以需要实现一个回退算法。论文中所提到的内容，这里就不再赘述了。论文主要是关注于多机共识的问题，但实际上逐个递减的回退算法性能存在着一定问题。在课程中，老师提到了一种快速回退的算法，需要 follower 在回复中附带冲突 log 的 term 和冲突term的第一个 index。一共有如下三种情况：

```
server1: 4 5 5    		
server2: 4 6 6 6 6	(leader)

server1: 4 4 4		
server2: 4 6 6 6		(leader)

server1: 4	
server2: 4 6 6 6 		(leader)
```

对于第一种情况，leader 根本没有冲突任期 term5 的 log，所以 leader 会直接把冲突任期的 index 作为 nextIndex。第二种情况，leader 含有 term4，那么就会选取冲突任期 term4 的最后一个 log 作为 prevLog。第三种情况，follower 没有 prevLog，那么 follower 就会返回最后一个 log 的 index，以及 term 为 0 or -1 作为标记。leader 会发送所有的 log。

这里需要注意，由于不可靠的网络，非常有可能接收到的回复任期已经过期了，同时，对于论文中 Figure 8 的情况，只能提交当前任期的 log，所以需要判断请求的任期是否与当前任期符合。

在更新 commitIndex 时，还是由于网络乱序的原因，commitIndex 只能单调增的判断也是必不可少的。

apply 时，由于 commitIndex 已经更新过了，更新的信息会搭载在心跳包中，广播给所有的 follower。

### Follower

复制 log 和心跳包的类型都是一样的，所以我通过 entries 的长度对两者进行区分。

#### 复制 log

对于复制 log，在 Leader append 成功后，会单独采取一个协程组，进行广播。Follower 对 prevLog 按照之前提到的方法进行判断，如果冲突，那么就会返回相应的信息。如果不冲突，因为乱序的原因，还需要判断是否是之前的信息。如果当前请求的所有 log 都已经在本地了，那么则不需要再进行更新。如果没有，那么进行更新。

#### 提交

提交的信息是搭载在心跳包中，只要 leader的 commitIndex 大于自己的，那么就可以提交。但存在这样一种情况，log 已经在多数 server 上复制成功，leader 发送提交信息，但当前 server 属于少数，并没有获取到 log。但是，根据 Raft 的性质，这也说明当前 server 的所有 log 都是可以提交的。所以，更新 commitIndex 要leaderCommit 和本地 log 长度两者取小。









