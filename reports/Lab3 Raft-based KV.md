# Lab3 Raft-based KV

Lab3主要是在Lab2实现的Raft基础上，实现KV存储的服务，主要提供`Put(key, value)`，`Append(key, arg)`和`Get(key)`这三种操作。

## 3A

Lab3A可以先从Client入手，因为Client提供的操作较为简单，并且实验中基于了一个假设：`It's OK to assume that a client will make only one call into a Clerk at a time.` 。所以每个client的请求都是线性执行的。

### client

#### 过滤重复请求

在实现Client的过程中，发送接收操作都比较常规，没有什么难度。而client较为关键的任务在于如何使得server可以进行重复请求的过滤。

因为client在发送失败或者其他原因的情况下，会不断重发重试。但可能出现server已经收到请求，但还未来得及返回给client，但client仍在不断重试。此时，需要server保持exactly-once的一致性，将已经收到的重复请求过滤掉。

为了实现exactly-once，那么最简单的方法就是为每个client的所有请求都分配一个单调递增的ID。当然，这个ID序列貌似可以实现在server端，由一个map维护，这在不发生leader替换的时候是可行的。但如果发生了leader更替，原来的递增序列就会都是，就会导致处理重复的请求。所以，最好的办法就是在client端实现递增序列，在server端，仍然使用一个map维护。

#### 寻找leader

client在发送请求的时候是不知道哪个server是leader的，所以需要对所有的server进行循环遍历，不是leader的server会拒绝请求，client就需要尝试下一个server。

这里主要需要实现的就是一个效率的优化。每次找到leader的时候，保存它的ID，在下一次发送请求的时候，直接从这个ID开始尝试，而不是从0开始。

### server

server端相比之下就比较复杂了，主要要实现的过程如下：

1. server接收client请求
2. server经过重复过滤后，调用Raft的Start提交请求
3. Raft commit成功，返回结果给server，server将操作应用到存储的数据上
4. server应用成功后，将结果返回给client

#### client -> server -> raft

client通过RPC分别调用server的Get和PutAppend两个接口，server需要先判断请求是否重复，并更新记录的最新MsgId，然后将请求封装成Op，调用Raft的Start，进行多节点备份。

我在实现这块的时候遇到一个坑，Get也需要提交。刚开始，我认为get操作可以直接从leader的Storage中查询，直接返回就可以了。重启恢复的时候，也不需要进行执行。但考虑这样一种情况：server发送给Raft一系列PutAppend后，发送Get，如果乱序执行，导致Get先返回到server，那么Get到的数据就是错误的，不符合linearable。而如果Get也需要提交，则可以将利用Raft的linearable，保证Get之前的所有Post一定早于Get执行。

#### raft -> server -> client

请求返回的逻辑要复杂很多，我个人的理解在于，从client到raft，是多个机器到一台机器，server要仅仅要处理对于本机的Raft的逻辑，而raft到client，则是相反的过程。server由本机的Raft输入，但却要分发到多个client上。

既然请求的传递顺序是从raft到client，那么我们也要重新从raft入手。Raft commit成功后，是通过applyChan这单独通道通信的。那么问题就来了，如何从一个通道中，分发到不同的client上？有一种方法是在leader中维护一个注册表，每个client的MsgId对应于一个notifyChan。server在接收到请求时，在注册表中创建一个特有的notifyChan，并等待。而在server启动的时候，创建一个协程，不断读取applyChan中的消息，读取到消息后，查询注册表，将消息传进notifyChan。此时，handler就会解除阻塞，将结果返回给client。

但我在实现的时候先参考了Github上的一种实现，即把notifyChan封装进Op里。实现完，分析的时候才意识到这种方法不是最优的，网络通信开销比较大，这也是我之后可以改进的。但逻辑相对简单一些。在使用第二种实现的时候，会出现多个server都想要发送的情况，所以要使用select default，防止后续重复发送阻塞applyLoop。

handler在收到notifyChan的消息或者超时后，就可以更新Storage或者查询，并返回结果给client。

## 3B

个人感觉，Lab3B是目前下来，除了Lab2C之外，最难的lab了。主要原因在于要对整个的Raft的结构进行重构，并且还要考虑server。

虽然逻辑比较复杂，但可以初步分解成以下几个方面：

1. server leader保存Snapshotserver
2. follower同步Snapshot
3. Raft Leader保存Snapshot
4. Raft follower同步Snapshot

### Server

作为leader，server端的主要任务就是检测log的大小，如果大于maxraftstate的话，就要调用Raft的函数进行snapshot。主要的实现方法就是采用ApplyMsg的CommandValid作为区分。正常的Raft提交消息，都为true，而为false的时候就是与snapshot有关的消息，Leader就需要检测log大小。这里有一个细节需要注意，因为Raft同时需要一个index参数，来判断对多少的log进行snapshot。所以，leader需要维护一个lastLogIndex变量，保存最新apply的index。

作为Follower时，因为follower的日志可能存在问题，所以server并不需要主动去检测log的大小，而是被动地接受同步snapshot的命令，再去更新就可以了。这里仍然可以将这一指令通过ApplyMsg搭载，当ApplyMsg的CommandValid为false，且Command为空时，那么这条消息就是给leader的，因为leader是snapshot的生产方，而不是消费者，这时leader就可以检查是否要产生snapshot了。而如果为Snapshot，那么这条消息就是给follower同步Snapshot的。

### Raft

LogEntry的Index在之前的lab中都形同虚设，和数组的index没有什么区别，无非是大一少一的区别。但由于使用了snapshot，那么log的第一个就不一定从0开始了。所以，对读取log，最好封装成一个函数，每次读取都要减去snapshot的index。

在Raft持久化和读取持久化的时候，都需要添加snapshot的信息，并且在启动的时候将lastApplied更新为snapshot的index。不仅如此，启动的时候还要发送一次消息，使得server更新snapshot。因为可能存在这样一种情况，所有机器重启后，leader提交第一个指令时，log的大小不足以进行一次snapshot，而之前的snapshot可能就没有进行同步就开始apply新提交的指令了。

无论是leader还是follower，在接收到进行snapshot的消息，对log进行裁剪的时候，有一点至关重要，论文中有所提及：

> Raft also includes a small amount of metadata
> in the snapshot: the last included index is the index of the
> last entry in the log that the snapshot replaces (the last entry the state machine had applied), and the last included
> term is the term of this entry. These are preserved to support the AppendEntries consistency check for the first log
> entry following the snapshot, since that entry needs a previous log index and term.

不能把整个log全部清空，需要把snapshot的最后一个log留下。不然leader之后发送preLogTerm 永远在follower里找不到，然后follower就会一直reply false。

到这里，就剩下leader和follower之间的同步了，也就是论文中所提到的InstallSnapshot RPC。这里可以通过心跳包搭载的方式就行同步，如果有需要同步snapshot，那么就搭载在AppendEntries里，如果没有，那么AppendEntries里的snapshot就为空。

这里有可能出现一些比较特殊的情况：

1. 可能follower掉线很久，缺乏很多日志，那么同步snapshot的时候，就需要将lastApplied和commitIndex都同步到和snapshot相同的地方。
2. 也有可能follower掉线期间新的日志的不够多，但snapshot的index大于了follower的lastApplied，所以在向server提交指令的时候，需要提前更新lastApplied到snapshot的index。
3. 由于1中情况的出现，AppendEntries中的prevLogIndex可能小于follower的snapshot的Index，那么就需要对AppendEntries中的entries进行阶段，只接受大于snapshot的log。







在实现的过程中，还有很多小坑，但改完以后没记住，之后想起来的话再来补充吧😁