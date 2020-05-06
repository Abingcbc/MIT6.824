package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"../labgob"
	"../labrpc"
	"bytes"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

//import "bytes"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	Persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()
	applyChan chan ApplyMsg       // for tester

	state    int //0: leader 1: candidate 2: follower
	leaderId int // leader ID

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	CurrentTerm int
	VotedFor    int
	Log         []LogEntry

	commitIndex int
	lastApplied int

	nVotes int // num of received votes

	// leader state
	nextIndex  []int
	matchIndex []int

	electionTimeout  int
	heartbeatTimeout int

	latestSendHeartbeatTime    int64
	latestReceiveHeartbeatTime int64

	electionTimeoutChan  chan bool // triggering an election
	heartbeatTimeoutChan chan bool // triggering a heartbeat

	snapshot Snapshot // snapshot
}

func (rf *Raft) getLogByIndexUnLock(index int) LogEntry {
	index -= rf.snapshot.Index
	if index < 0 {
		return LogEntry{
			Index: rf.snapshot.Index,
			Term:  rf.snapshot.Term,
			Log:   nil,
		}
	} else {
		if index >= len(rf.Log) {
			indexs := make([]int, len(rf.Log))
			for i, log := range rf.Log {
				indexs[i] = log.Index
			}
			//DPrintf("[getLogByIndexUnLock]: Id %v LastIndex %v GetIndex %v CommitIndex %v SnapIndex %v Start %v",
			//	rf.me, rf.getLastLogUnLock().Index, index+rf.snapshot.Index,
			//	rf.commitIndex, rf.snapshot.Index, indexs)
		}
		return rf.Log[index]
	}
}

func (rf *Raft) getLastLogUnLock() LogEntry {
	if len(rf.Log) == 0 {
		return LogEntry{
			Index: rf.snapshot.Index,
			Term:  rf.snapshot.Term,
			Log:   nil,
		}
	} else {
		return rf.Log[len(rf.Log)-1]
	}
}

// for concurrency access safety
// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state == 0 {
		isleader = true
	} else {
		isleader = false
	}
	term = rf.CurrentTerm
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.CurrentTerm)
	e.Encode(rf.VotedFor)
	e.Encode(rf.Log)
	e.Encode(rf.snapshot.Index)
	e.Encode(rf.snapshot.Term)
	e.Encode(rf.commitIndex)
	data := w.Bytes()
	rf.Persister.SaveStateAndSnapshot(data, rf.snapshot.Storage)
	// snapshot is elder than server's storage
	if rf.lastApplied > rf.snapshot.Index {
		return
	}
	// only trigger StateSize check
	applyMsg := ApplyMsg{
		CommandValid: false,
		Command:      nil,
		CommandIndex: 0,
	}
	rf.applyChan <- applyMsg
	//DPrintf("[persister]: Id %v Term %v SnapshotIndex %v Size %v",
	//	rf.me, rf.CurrentTerm, rf.snapshot.Index, rf.Persister.RaftStateSize())
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm int
	var voteFor int
	var commitIndex int
	var log []LogEntry
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&voteFor) != nil ||
		d.Decode(&log) != nil ||
		d.Decode(&rf.snapshot.Index) != nil ||
		d.Decode(&rf.snapshot.Term) != nil ||
		d.Decode(&commitIndex) != nil {
		DPrintf("[readPersist] %v fail", rf.me)
	} else {
		rf.CurrentTerm = currentTerm
		rf.VotedFor = voteFor
		rf.Log = log
		if len(rf.Log) == 0 {
			rf.Log = append(rf.Log, LogEntry{
				Index: rf.snapshot.Index,
				Term:  rf.snapshot.Term,
				Log:   nil,
			})
		}
		rf.commitIndex = commitIndex
		rf.lastApplied = rf.snapshot.Index
		DPrintf("[readPersist] %v success SnapIndex %v CommitIndex %v",
			rf.me, rf.snapshot.Index, rf.commitIndex)
	}
	rf.snapshot.Storage = rf.Persister.ReadSnapshot()
	if rf.snapshot.Index > 0 {
		applyMsg := ApplyMsg{
			CommandValid: false,
			Command:      rf.snapshot,
			CommandIndex: rf.snapshot.Index,
		}
		rf.applyChan <- applyMsg
	}
}

//
// RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int // candidate's term
	CandidateId  int // candidate's id
	LastLogIndex int // index of candidate's last index
	LastLogTerm  int // term of LastLogIndex
}

//
// RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int  // voter's current term
	VoteGranted bool // if vote for candidate
}

type LogEntry struct {
	Index int
	Term  int
	Log   interface{}
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.CurrentTerm <= args.Term {
		if rf.CurrentTerm < args.Term {
			rf.CurrentTerm = args.Term // update to newest term
			rf.VotedFor = -1           // allow server to vote for other
			rf.state = 2               // turn to follower
			rf.persist()
		}
		// server is able to vote or has voted for the candidate
		if rf.VotedFor == -1 || rf.VotedFor == args.CandidateId {
			lastLogIndex := rf.getLastLogUnLock().Index
			lastLogTerm := rf.getLastLogUnLock().Term
			// if candidate's log is more up-to-date than server
			if lastLogTerm < args.LastLogTerm ||
				(lastLogTerm == args.LastLogTerm && lastLogIndex <= args.LastLogIndex) {
				rf.VotedFor = args.CandidateId
				rf.resetElectionTimer()
				rf.state = 2
				DPrintf("[RequestVote]: Id %v Term %v vote for %v", rf.me,
					rf.CurrentTerm, args.CandidateId)
				rf.persist()
				reply.Term = rf.CurrentTerm
				reply.VoteGranted = true
				return
			}
		}
	}
	reply.Term = rf.CurrentTerm
	reply.VoteGranted = false
}

func (rf *Raft) resetElectionTimer() {
	rand.Seed(time.Now().UnixNano())
	rf.electionTimeout = rf.heartbeatTimeout*5 + rand.Intn(150)
	rf.latestReceiveHeartbeatTime = time.Now().UnixNano()
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

type AppendEntriesArgs struct {
	Term         int        // leader's term
	LeaderId     int        // leader Id
	PrevLogIndex int        // index of the log before the new one
	PrevLogTerm  int        // term of PrevLogIndex
	Entries      []LogEntry // new logs
	LeaderCommit int        // leader's commitIndex
	LogSnapshot  Snapshot   // snapshot of leader
}

type AppendEntriesReply struct {
	Term          int  // follower's current term
	Success       bool // if matching PrevLogIndex and PrevLogTerm
	ConflictIndex int  // the first index for the conflicting term
	ConflictTerm  int  // the term of conflicting log
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	//DPrintf("[AppendEntries]: Id %v Term %v receive from %v Term %v",
	//	rf.me, rf.CurrentTerm, args.LeaderId, args.Term)
	// request term is smaller than current term, directly refuse
	if rf.CurrentTerm <= args.Term {
		if rf.CurrentTerm < args.Term {
			// out-of-date
			// turn to follower and sync term
			rf.CurrentTerm = args.Term
			rf.resetElectionTimer()
			rf.VotedFor = -1
			if rf.state != 2 {
				DPrintf("[AppendEntries]: Id %v Term %v turn to follower",
					rf.me, rf.CurrentTerm)
			}
			rf.state = 2
			rf.persist()
		}
		offsetIndex := rf.snapshot.Index
		// if PrevLogIndex is in snapshot
		if args.PrevLogIndex < rf.snapshot.Index {
			args.PrevLogIndex = rf.snapshot.Index
			args.PrevLogTerm = rf.snapshot.Term
			start := 0
			for ; start < len(args.Entries); start++ {
				if args.Entries[start].Index > rf.snapshot.Index {
					break
				}
			}
			args.Entries = args.Entries[start:]
		}
		// if PrevLog is consistent
		if rf.getLastLogUnLock().Index >= args.PrevLogIndex &&
			rf.getLogByIndexUnLock(args.PrevLogIndex).Term == args.PrevLogTerm {
			rf.state = 2
			isMatch := true
			nextIndex := args.PrevLogIndex + 1
			end := rf.getLastLogUnLock().Index
			for i := 0; i < len(args.Entries); i++ {
				if end < nextIndex+i ||
					rf.getLogByIndexUnLock(nextIndex+i).Term != args.Entries[i].Term {
					isMatch = false
					break
				}
			}
			// copy entries to log
			if !isMatch {
				entries := make([]LogEntry, len(args.Entries))
				copy(entries, args.Entries)
				if nextIndex-1 == rf.getLastLogUnLock().Index {
					rf.Log = append(rf.Log, entries...)
				} else {
					rf.Log = append(rf.Log[:nextIndex-offsetIndex], entries...)
				}
			}
			indexOfLastNewEntry := args.PrevLogIndex + len(args.Entries)
			if args.LeaderCommit > rf.commitIndex {
				rf.commitIndex = args.LeaderCommit
				if rf.commitIndex > indexOfLastNewEntry {
					rf.commitIndex = indexOfLastNewEntry
				}
				// apply logs
				go rf.apply()
			}
			rf.updateSnapshotUnLock(args.LogSnapshot)
			rf.resetElectionTimer()
			rf.persist()
			reply.Term = rf.CurrentTerm
			reply.Success = true
			return
		} else {
			nextIndex := args.PrevLogIndex + 1
			rf.resetElectionTimer()

			// miss some logs before
			if rf.getLastLogUnLock().Index < nextIndex {
				reply.ConflictTerm = 0
				reply.ConflictIndex = rf.getLastLogUnLock().Index
			} else {
				// PrevLog term conflict
				reply.ConflictTerm = rf.getLogByIndexUnLock(args.PrevLogIndex).Term
				reply.ConflictIndex = args.PrevLogIndex
				// find the last index of the term before conflicted one
				for i := reply.ConflictIndex - 1; i > offsetIndex; i-- {
					if rf.getLogByIndexUnLock(i).Term != reply.ConflictTerm {
						break
					} else {
						reply.ConflictIndex--
					}
				}
			}
		}
	}
	reply.Term = rf.CurrentTerm
	reply.Success = false
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// for leader to quickly backward to the nextIndex
func (rf *Raft) getNextIndex(reply AppendEntriesReply, nextIndex int) int {
	// rf is locked outside the function
	offsetIndex := rf.snapshot.Index
	if reply.ConflictTerm == 0 {
		// follower log is less than nextIndex
		nextIndex = reply.ConflictIndex
	} else {
		conflictIndex := reply.ConflictIndex
		conflictTerm := rf.getLogByIndexUnLock(conflictIndex).Term

		// only when leader's conflictTerm larger than follower's
		// can find log with same term
		if conflictTerm >= reply.ConflictTerm {
			// find one index of conflictTerm
			// may not be the last one
			for ; conflictIndex > offsetIndex; conflictIndex-- {
				if rf.getLogByIndexUnLock(conflictIndex).Term == reply.ConflictTerm {
					break
				}
			}
			if conflictIndex != 0 {
				// find the last index of conflictTerm
				for i := conflictIndex + 1; i < nextIndex; i++ {
					if rf.getLogByIndexUnLock(i).Term != reply.ConflictTerm {
						break
					}
					conflictIndex += 1
				}
				nextIndex = conflictIndex + 1
			} else {
				// no conflictTerm
				nextIndex = reply.ConflictIndex
			}
		} else {
			nextIndex = reply.ConflictIndex
		}
	}
	return nextIndex
}

// broadcast AppendEntries
// index: new log
// term: current term
func (rf *Raft) broadcastAppendEntries(index int, term int, commitIndex int,
	nReplica int) {
	majority := len(rf.peers)/2 + 1
	isAgree := false

	// only leader can send AppendEntries
	if _, isLeader := rf.GetState(); isLeader == false {
		return
	}

	rf.mu.Lock()
	// avoid term changed
	if rf.CurrentTerm != term {
		rf.mu.Unlock()
		return
	}
	rf.mu.Unlock()

	for i, _ := range rf.peers {
		if i == rf.me {
			continue
		}
		// send AppendEntries to i
		go func(i int, rf *Raft) {
		retry:
			if _, isLeader := rf.GetState(); isLeader == false {
				return
			}

			rf.mu.Lock()
			// avoid term changed
			if rf.CurrentTerm != term || index < rf.lastApplied {
				rf.mu.Unlock()
				return
			}
			offsetIndex := rf.snapshot.Index
			nextIndex := rf.nextIndex[i]
			prevLogIndex := nextIndex - 1
			prevLogTerm := rf.getLogByIndexUnLock(prevLogIndex).Term

			entries := make([]LogEntry, 0)
			// if not, the AppendEntries will be empty and become a heartbeat
			if nextIndex < index+1 {
				if nextIndex-offsetIndex < 0 {
					entries = rf.Log[:index-offsetIndex+1]
				} else {
					entries = rf.Log[nextIndex-offsetIndex : index-offsetIndex+1]
				}
			}
			args := AppendEntriesArgs{
				Term:         term,
				LeaderId:     rf.me,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      entries,
				LeaderCommit: commitIndex,
				LogSnapshot:  rf.snapshot,
			}
			rf.mu.Unlock()
			//if len(entries) > 0 {
			//	DPrintf("[broadcastAppendEntries]: Id %v to %v %v",
			//		rf.me, i, args)
			//}
			var reply AppendEntriesReply

			ok := rf.sendAppendEntries(i, &args, &reply)

			// fail to send, abandon
			if ok == false {
				return
			}
			// receive out-of-date reply
			rf.mu.Lock()
			if rf.CurrentTerm != args.Term {
				rf.mu.Unlock()
				return
			}
			rf.mu.Unlock()

			if reply.Success == false {
				rf.mu.Lock()
				//if len(args.Entries) > 0 {
				//	DPrintf("[broadcastAppendEntries]: Id %v Term %v fail",
				//		rf.me, rf.CurrentTerm)
				//}

				// 这里比较的是args中的term和reply的term，
				// 不比较rf.CurrentTerm的原因在于，等待reply返回的这一段时间里，
				// 可能CurrentTerm发生了变化，而这种变化有可能是错误的
				// 设想这样一种情况：
				// peer1作为leader，也就是本函数的执行者，请求阻塞在接受者peer2之间
				// 在这段时间内，peer1因为某种原因，失去leader身份，接着peer1在一段时间网络中断，
				// 由于选举出了新的leader，所以peer2的CurrentTerm一定增加了，
				// 此时peer2收到请求，拒绝然后返回，但同样地response也阻塞在中途
				// 而由于peer1网络中断，其一直在不停选举，term会增加到很大，
				// 然后peer1网络恢复正常，接收到peer2的回复，
				// 但此时显然peer1已经不是leader，即使其CurrentTerm大，但也无资格进行同步操作
				if args.Term < reply.Term {
					DPrintf("[broadcastAppendEntries]: Id %v Term %v turn to Term %v follower %v",
						rf.me, rf.CurrentTerm, reply.Term, i)
					rf.CurrentTerm = reply.Term
					rf.VotedFor = -1
					rf.state = 2
					rf.persist()
					rf.mu.Unlock()
					return
				} else {
					// backward to non-conflict index
					nextIndex := rf.getNextIndex(reply, nextIndex)
					rf.nextIndex[i] = nextIndex
					rf.mu.Unlock()
					goto retry
				}
			} else {
				//if len(args.Entries) > 0 {
				//	DPrintf("[broadcastAppendEntries]: Id %v Term %v succeed from %v",
				//		rf.me, rf.CurrentTerm, i)
				//}
				// success replicate to peer
				rf.mu.Lock()
				// avoid out-of-order update nextIndex
				if rf.nextIndex[i] < index+1 {
					rf.nextIndex[i] = index + 1
					rf.matchIndex[i] = index
				}
				nReplica++
				if isAgree == false && rf.state == 0 && nReplica >= majority {
					isAgree = true
					// can only commit current term's log
					if rf.commitIndex < index && rf.getLogByIndexUnLock(index).Term == rf.CurrentTerm {
						rf.commitIndex = index
						// notify other peers to commit
						go rf.broadcastHeartBeat()
						go rf.apply()
					}
				}
				rf.mu.Unlock()
			}
		}(i, rf)
	}
}

func (rf *Raft) apply() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	commitIndex := rf.commitIndex
	if rf.lastApplied < rf.snapshot.Index {
		msg := ApplyMsg{
			CommandValid: false,
			Command:      rf.snapshot,
			CommandIndex: 0,
		}
		rf.applyChan <- msg
		DPrintf("[raft.apply]: Id %v update lastApplied %v to %v",
			rf.me, rf.lastApplied, rf.snapshot.Index)
		rf.lastApplied = rf.snapshot.Index
	}
	if rf.commitIndex < rf.snapshot.Index {
		return
	}
	lastApplied := rf.lastApplied
	DPrintf("[raft.apply]: Id %v commit from %v to %v",
		rf.me, rf.lastApplied+1, rf.commitIndex)
	if lastApplied != commitIndex {
		for i := lastApplied + 1; i <= commitIndex; i++ {
			applyMsg := ApplyMsg{
				CommandValid: true,
				Command:      rf.getLogByIndexUnLock(i).Log,
				CommandIndex: i,
			}
			rf.lastApplied = i
			rf.applyChan <- applyMsg
		}
	}
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).
	if term, isLeader = rf.GetState(); isLeader {
		rf.mu.Lock()
		logEntry := LogEntry{
			Term:  rf.CurrentTerm,
			Log:   command,
			Index: rf.snapshot.Index + len(rf.Log),
		}
		rf.Log = append(rf.Log, logEntry)
		index = logEntry.Index
		nReplica := 1
		rf.persist()

		rf.latestSendHeartbeatTime = time.Now().UnixNano()
		DPrintf("[Start]: Id %v Term %d new command %v", rf.me,
			rf.CurrentTerm, index)
		go rf.broadcastAppendEntries(index, rf.CurrentTerm, rf.commitIndex, nReplica)
		rf.mu.Unlock()
	}
	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) electionCheckLoop() {
	for !rf.killed() {
		if _, isLeader := rf.GetState(); !isLeader {
			rf.mu.Lock()
			elapseTime := time.Now().UnixNano() - rf.latestReceiveHeartbeatTime
			// election timeout
			if int(elapseTime/int64(time.Millisecond)) >= rf.electionTimeout {
				rf.electionTimeoutChan <- true
			}
			rf.mu.Unlock()
		}
		time.Sleep(time.Millisecond * 10)
	}
}

func (rf *Raft) startOneElection() {
	rf.mu.Lock()
	// turn to candidate
	rf.state = 1
	rf.CurrentTerm++
	rf.VotedFor = rf.me
	rf.persist()

	nVotes := 1
	rf.resetElectionTimer()
	rf.mu.Unlock()

	go func(nVotes *int, rf *Raft) {
		majority := len(rf.peers)/2 + 1
		for i, _ := range rf.peers {
			if i == rf.me {
				continue
			}

			rf.mu.Lock()
			lastLogIndex := rf.getLastLogUnLock().Index
			lastLogTerm := rf.getLastLogUnLock().Term
			args := RequestVoteArgs{
				Term:         rf.CurrentTerm,
				CandidateId:  rf.me,
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			}
			rf.mu.Unlock()

			var reply RequestVoteReply
			go func(i int, rf *Raft, args *RequestVoteArgs, reply *RequestVoteReply) {
				ok := rf.sendRequestVote(i, args, reply)
				if ok {
					// handle out-of-date reply
					rf.mu.Lock()
					if rf.CurrentTerm != args.Term {
						rf.mu.Unlock()
						return
					}
					rf.mu.Unlock()
					if !reply.VoteGranted {
						rf.mu.Lock()
						defer rf.mu.Unlock()
						// fail
						if rf.CurrentTerm < reply.Term {
							rf.CurrentTerm = reply.Term
							rf.VotedFor = -1
							rf.state = 2
							rf.persist()
						}
					} else {
						// win the vote
						rf.mu.Lock()
						*nVotes++
						if rf.state == 1 && *nVotes >= majority {
							log.Printf("[startOneElection]: Id %v Term %v win the leader",
								rf.me, rf.CurrentTerm)

							for i := 0; i < len(rf.peers); i++ {
								rf.nextIndex[i] = len(rf.Log) + rf.snapshot.Index
								rf.matchIndex[i] = rf.snapshot.Index
							}
							rf.leaderId = rf.me
							rf.state = 0
							go rf.broadcastHeartBeat()
							rf.persist()
						}
						rf.mu.Unlock()
					}
				}
			}(i, rf, &args, &reply)
		}
	}(&nVotes, rf)

}

// leader send heartbeat to all peers to extend lease
func (rf *Raft) heartBeatLoop() {
	for !rf.killed() {
		if _, isLeader := rf.GetState(); isLeader {
			rf.mu.Lock()
			elapseTime := time.Now().UnixNano() - rf.latestSendHeartbeatTime
			if int(elapseTime/int64(time.Millisecond)) >= rf.heartbeatTimeout {
				rf.heartbeatTimeoutChan <- true
			}
			rf.mu.Unlock()
		}
		time.Sleep(time.Millisecond * 10)
	}
}

func (rf *Raft) broadcastHeartBeat() {
	if _, isLeader := rf.GetState(); !isLeader {
		return
	}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.latestSendHeartbeatTime = time.Now().UnixNano()
	index := rf.getLastLogUnLock().Index
	nReplica := 1
	go rf.broadcastAppendEntries(index, rf.CurrentTerm, rf.commitIndex, nReplica)
}

func (rf *Raft) eventLoop() {
	for {
		select {
		case <-rf.electionTimeoutChan:
			go rf.startOneElection()
		case <-rf.heartbeatTimeoutChan:
			go rf.broadcastHeartBeat()
		}
	}
}

type Snapshot struct {
	Index   int
	Term    int
	Storage []byte
}

func (rf *Raft) SaveSnapshot(index int, Storage []byte) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	offsetIndex := rf.snapshot.Index
	if index > rf.snapshot.Index {
		log.Printf("[SaveSnapshot]: Id %v Index %v",
			rf.me, index)
		// save snapshot
		rf.snapshot.Term = rf.getLogByIndexUnLock(index).Term
		rf.snapshot.Index = index
		rf.snapshot.Storage = Storage
		// discard log
		// save the snapshot-index log
		rf.Log = rf.Log[index-offsetIndex:]
		rf.persist()
	}
}

func (rf *Raft) updateSnapshotUnLock(snapshot Snapshot) {
	// update snapshot
	if snapshot.Index > rf.snapshot.Index {
		rf.snapshot = snapshot
		start := 0
		for ; start < len(rf.Log); start++ {
			if rf.Log[start].Index == snapshot.Index {
				break
			}
		}
		if start == len(rf.Log) {
			rf.Log = append(make([]LogEntry, 0), LogEntry{
				Index: rf.snapshot.Index,
				Term:  rf.snapshot.Term,
				Log:   nil,
			})
		} else {
			rf.Log = rf.Log[start:]
		}
		log.Printf("[updateSnapshotUnLock]: Id %v SnapshotIndex %v LastIndex %v "+
			"CommitIndex %v Start %v Length %v",
			rf.me, rf.snapshot.Index, rf.getLastLogUnLock().Index,
			rf.commitIndex, rf.Log[0].Index, len(rf.Log))
	}
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.Persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.applyChan = applyCh
	rf.state = 2
	rf.leaderId = -1
	rf.heartbeatTimeout = 110
	rf.resetElectionTimer()
	rf.electionTimeoutChan = make(chan bool)
	rf.heartbeatTimeoutChan = make(chan bool)
	rf.VotedFor = -1
	rf.Log = make([]LogEntry, 0)
	rf.Log = append(rf.Log, LogEntry{
		Index: 0,
		Term:  0,
		Log:   nil,
	})

	size := len(rf.peers)
	rf.nextIndex = make([]int, size)
	rf.matchIndex = make([]int, size)
	rf.snapshot = Snapshot{
		Index:   0,
		Term:    0,
		Storage: nil,
	}
	rf.CurrentTerm = rf.snapshot.Term
	rf.commitIndex = rf.snapshot.Index
	rf.lastApplied = rf.snapshot.Index

	go rf.electionCheckLoop()
	go rf.heartBeatLoop()
	go rf.eventLoop()

	rf.mu.Lock()
	rf.readPersist(persister.ReadRaftState())
	rf.mu.Unlock()

	return rf
}
