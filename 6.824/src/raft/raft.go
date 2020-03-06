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
	"bytes"
	"math/rand"
	"sync"
	"time"
)
import "sync/atomic"
import "../labrpc"

//import "bytes"
import "../labgob"

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
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()
	applyChan chan ApplyMsg       // for tester


	state int //0: leader 1: candidate 2: follower
	isReceivedHeart bool // whether has received a heartbeat during a election check loop

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentTerm int
	votedFor    int
	log         []LogEntry

	commitIndex int
	lastApplied int

	// leader state
	nextIndex  []int
	matchIndex []int
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
	term = rf.currentTerm
	//DPrintf("%v GetState %v %v", rf.me, term, isleader)
	return term, isleader
}

func (rf *Raft) getOnlyState() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.state
}

func (rf *Raft) setState(state int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.state = state
}

func (rf *Raft) getIsReceivedHeart() bool {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.isReceivedHeart
}

func (rf *Raft) setIsReceivedHeart(isReceivedHeart bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.isReceivedHeart = isReceivedHeart
}

func (rf *Raft) getCurrentTerm() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm
}

func (rf *Raft) setCurrentTerm(newTerm int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if newTerm < 0 {
		rf.currentTerm++
	} else {
		rf.currentTerm = newTerm
	}
}

func (rf *Raft) getVoteFor() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.votedFor
}

func (rf *Raft) setVoteFor(voteFor int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.votedFor = voteFor
}

func (rf *Raft) getLog(index int) LogEntry {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// Log index start from 1 but slice start from 0
	if len(rf.log) == 0 || index == 0{
		return LogEntry{
			Index: 0,
			Term:  0,
			Log:   nil,
		}
	}
	return rf.log[index-1]
}

func (rf *Raft) appendLog(logEntry LogEntry) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.log = append(rf.log, logEntry)
}

func (rf *Raft) getAllLog() []LogEntry {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.log
}

func (rf *Raft) getLogLength() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return len(rf.log)
}

func (rf *Raft) getCommitIndex() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.commitIndex
}

func (rf *Raft) setCommitIndex(commitIndex int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.commitIndex = commitIndex
}

func (rf *Raft) getLastApplied() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.lastApplied
}

func (rf *Raft) setLastApplied(lastApplied int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.lastApplied = lastApplied
}

func (rf *Raft) getNextIndex(peer int) int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.nextIndex[peer]
}

func (rf *Raft) setNextIndex(peer int, value int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if value == -1 {
		rf.nextIndex[peer]--
	} else {
		rf.nextIndex[peer] = value
	}
}

func (rf *Raft) getMatchIndex(peer int) int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.matchIndex[peer]
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
	e.Encode(rf.getCurrentTerm())
	e.Encode(rf.getVoteFor())
	e.Encode(rf.getAllLog())
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
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
	var log []LogEntry
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&voteFor) != nil ||
		d.Decode(&log) != nil {
		DPrintf("%v read persistent state fail", rf.me)
	} else {
		DPrintf("%v read persistent state success", rf.me)
		rf.currentTerm = currentTerm
		rf.votedFor = voteFor
		rf.log = log
		DPrintf("%v %v %v %v", rf.me,
			rf.currentTerm, rf.votedFor, rf.log)
	}
}

//
// RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

//
// RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term 		  int
	VoteGranted   bool
}

type LogEntry struct {
	Index int
	Term  int
	Log   interface{}
}

type AppendEntriesArgs struct {
	Term int
	LeaderId int
	PrevLogIndex int
	PrevLogTerm int
	Entries []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term int
	Success bool
	ConflictIndex int
	ConflictTerm  int
}

type Vote struct {
	sync.Mutex
	voteNumber int
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	reply.Term = rf.getCurrentTerm()
	// If server has voted for other server (not self), it cannot vote for a second one
	if rf.getCurrentTerm() < args.Term {
		// if not in same term, definitely not up-to-date
		rf.setCurrentTerm(args.Term)
		rf.setState(2)
		if args.LastLogTerm < rf.getLog(rf.getLogLength()).Term {
			reply.VoteGranted = false
			return
		} else if args.LastLogTerm == rf.getLog(rf.getLogLength()).Term {
			// If in same term, index decide whether up-to-date
			if args.LastLogIndex < rf.getLog(rf.getLogLength()).Index {
				reply.VoteGranted = false
				return
			}
		}
		rf.setVoteFor(args.CandidateId)
		rf.persist()
		reply.VoteGranted = true
		rf.setIsReceivedHeart(true)
		DPrintf("%v vote for %v (%v vs %v)", rf.me,
			args.CandidateId, rf.getCurrentTerm(),
			args.Term)
	} else {
		reply.VoteGranted = false
	}
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

func (rf *Raft) apply() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	lastApplied := rf.lastApplied
	for i := lastApplied+1; i <= rf.commitIndex; i++ {
		applyEntry := rf.log[i-1]
		applyMsg := ApplyMsg{
			CommandValid: true,
			Command:      applyEntry.Log,
			CommandIndex: applyEntry.Index,
		}
		//DPrintf("%v apply %v command %v",
		//	rf.me, applyEntry.Index, applyEntry.Log)
		rf.applyChan <- applyMsg
		rf.lastApplied++
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
	if rf.state != 0 {
		isLeader = false
	} else {
		rf.mu.Lock()
		index = len(rf.log) + 1
		term = rf.currentTerm
		newEntry := LogEntry{
			Index: index,
			Term:  term,
			Log:   command,
		}
		rf.log = append(rf.log, newEntry)
		rf.mu.Unlock()
		rf.persist()
		go rf.replicateCommandLoop(newEntry)
		if index < 100 {
			DPrintf("%v receive command %v-%v: %v", rf.me,
				newEntry.Index, newEntry.Term, newEntry.Log)
		}
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

func (rf *Raft) electionCheckLoop()  {
	for !rf.killed() {
		// not leader and not receive heartbeat
		if rf.getOnlyState() != 0 && !rf.getIsReceivedHeart() {
			positiveVote := Vote{voteNumber: 0}
			DPrintf("%v start a election term\n", rf.me)
			rf.setCurrentTerm(-1)
			rf.setState(1)
			// vote for self
			rf.setVoteFor(rf.me)
			rf.persist()
			for i := 0; i < len(rf.peers); i++ {
				if i == rf.me {
					continue
				}
				if rf.getOnlyState() != 1 {
					break
				}
				// avoid bottleneck
				go func(index int) {
					request := RequestVoteArgs{
						Term:         rf.getCurrentTerm(),
						CandidateId:  rf.me,
						LastLogIndex: rf.getLog(rf.getLogLength()).Index,
						LastLogTerm:  rf.getLog(rf.getLogLength()).Term,
					}
					reply := RequestVoteReply{}
					result := rf.sendRequestVote(index, &request, &reply)
					if result && reply.VoteGranted {
						positiveVote.Lock()
						positiveVote.voteNumber++
						positiveVote.Unlock()
						return
					}
					if !result {
						//DPrintf("%v get vote timeout from %v", rf.me, index)
						return
					}
					// if leader has been elected, there is no need to compete
					if rf.getOnlyState() != 1 || rf.getCurrentTerm() != request.Term{
						return
					}
					if !reply.VoteGranted {
						if reply.Term < rf.getCurrentTerm() {
							DPrintf("%v exceed term and reset to %v",
								rf.me, reply.Term)
							rf.setCurrentTerm(reply.Term)
							rf.persist()
						}
					}
				}(i)
			}
			// wait for vote, timeout ones will not include
			time.Sleep(100*time.Millisecond)
			// Win majority vote
			// If server still is a candidate, it votes for itself
			positiveVote.Lock()
			vote := positiveVote.voteNumber
			positiveVote.Unlock()
			if rf.getOnlyState() == 1 && vote >= int(len(rf.peers)/2) {
				DPrintf("%v win the leader\n", rf.me)
				rf.setState(0)
				rf.persist()
				go rf.heartBeatLoop()
			}
			rf.setIsReceivedHeart(false)
			// Random timeout
			time.Sleep(time.Duration(rand.Intn(150) + 250)*time.Millisecond)
		} else {
			rf.setIsReceivedHeart(false)
			// Election timeout
			time.Sleep(200*time.Millisecond)
		}
	}
}

func (rf *Raft) sendHeartBeat(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.RequestHeartBeat", args, reply)
	return ok
}

func (rf *Raft) replicateCommandLoop(command LogEntry) {
	receiveSuccess := Vote{voteNumber: 0}
	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me {
			continue
		}
		var entries []LogEntry
		for j := rf.getNextIndex(i); j <= command.Index; j++ {
			entries = append(entries, rf.getLog(j))
		}
		args := AppendEntriesArgs{
			Term:         rf.getCurrentTerm(),
			LeaderId:     rf.me,
			PrevLogIndex: rf.getNextIndex(i)-1,
			PrevLogTerm:  rf.getLog(rf.getNextIndex(i)-1).Term,
			Entries:      entries,
			LeaderCommit: rf.getCommitIndex(), // not use by follower here
		}
		reply := AppendEntriesReply{}
		go func(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) {
			response := rf.sendHeartBeat(server, args, reply)
			for !rf.killed() && (!response || !reply.Success) {
				if !response {
					response = rf.sendHeartBeat(server, args, reply)
					//return
				} else if !reply.Success {
					// There are two reasons for false result
					// One is leader fall behind
					if reply.Term > rf.getCurrentTerm() {
						DPrintf("%v fall behind & lose leader because %v",
							rf.me, server)
						rf.setState(2)
						rf.setCurrentTerm(reply.Term)
						return
					}
					if rf.getOnlyState() != 0 || args.Term != rf.getCurrentTerm() {
						//DPrintf("abandon command %v to %v (%v-%v)",
						//	entries[0].Log, server, args.Term, reply.Term)
						return
					}
					// One is follower doesn't have entries before
					DPrintf("%v doesn't have entries before %v", server,
						args.PrevLogIndex)
					rf.mu.Lock()
					if reply.ConflictTerm == 0 {
						// 没有该log，那么从冲突开始的所有log都需要发送
						rf.nextIndex[server] = reply.ConflictIndex
					} else {
						// 有该log，但是term不同
						conflictIndex := reply.ConflictIndex
						conflictTerm := rf.log[conflictIndex-1].Term
						// 如果leader遗漏了该term的log，向前搜索等于该term的
						if conflictTerm >= reply.ConflictTerm {
							for i := conflictIndex; i > 0; i-- {
								if rf.log[i-1].Term == reply.ConflictTerm {
									break
								}
								conflictIndex -= 1
							}
							// conflictIndex不为0，leader的log中存在同任期的entry
							if conflictIndex != 0 {
								// 向后搜索，使得conflictIndex为最后一个任期等于reply.ConflictTerm的entry
								nextIndex := rf.nextIndex[server]
								for i := conflictIndex+1; i < nextIndex; i++ {
									if rf.log[i-1].Term != reply.ConflictTerm {
										break
									}
									conflictIndex += 1
								}
								rf.nextIndex[server] = conflictIndex + 1
							} else {	// conflictIndex等于0，说明不存在同任期的entry
								rf.nextIndex[server] = reply.ConflictIndex
							}
						}
					}
					args.Entries = rf.log[rf.nextIndex[server]-1:command.Index]
					rf.mu.Unlock()
					args.PrevLogIndex = args.Entries[0].Index-1
					args.PrevLogTerm = rf.getLog(args.PrevLogIndex).Term
					response = rf.sendHeartBeat(server, args, reply)
				}
			}
			//DPrintf("%v replicate success", server)
			if rf.killed() {
				return
			}
			receiveSuccess.Lock()
			rf.nextIndex[server] = command.Index+1
			receiveSuccess.voteNumber++
			majority := int(len(rf.peers)/2)
			temp := receiveSuccess.voteNumber
			receiveSuccess.Unlock()
			if temp == majority {
				// replicate successfully on majority, start commit
				if command.Term == rf.getCurrentTerm() {
					// out-of-order request
					if command.Index > rf.getCommitIndex() {
						rf.setCommitIndex(command.Index)
						//DPrintf("leader %v start apply to %v", rf.me,
						//	command.Index)
						rf.apply()
						rf.persist()
					}
				}
			}
		}(i, &args, &reply)
	}
}

// leader send heartbeat to all peers to extend lease
func (rf *Raft) heartBeatLoop() {
	for !rf.killed() {
		if rf.getOnlyState() != 0 {
			break
		}
		for i := 0; i < len(rf.peers); i++ {
			if i == rf.me {
				continue
			}
			go func(index int) {
				args := AppendEntriesArgs{
					Term:         rf.currentTerm,
					LeaderId:     rf.me,
					PrevLogIndex: rf.getLog(rf.getLogLength()).Index,
					PrevLogTerm:  rf.getLog(rf.getLogLength()).Term,
					Entries:      nil,
					LeaderCommit: rf.getCommitIndex(),
				}
				reply := AppendEntriesReply{}
				result := rf.sendHeartBeat(index, &args, &reply)
				if result {
					if !reply.Success {
						rf.setState(2)
						rf.setCurrentTerm(reply.Term)
						rf.setVoteFor(-1)
						rf.persist()
					}
				}
			}(i)
		}
		// period span (10 per second)
		time.Sleep(100*time.Millisecond)
	}
}

// handle heartbeat and replicate log entries
func (rf *Raft) RequestHeartBeat(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	reply.Term = rf.getCurrentTerm()
	// term < currentTerm
	if args.Term < rf.getCurrentTerm() {
		reply.Success = false
		return
	} else if rf.getCurrentTerm() < args.Term {
		reply.Success = true
		rf.setCurrentTerm(args.Term)
		rf.setState(2)
		rf.setIsReceivedHeart(true)
		rf.persist()
	}
	// heartbeat
	if len(args.Entries) == 0 {
		reply.Success = true
		rf.setIsReceivedHeart(true)
		rf.setCurrentTerm(args.Term)
		rf.setState(2)
		rf.setVoteFor(-1)
		rf.persist()
		// piggybacking commit message
		if args.LeaderCommit > rf.getCommitIndex() {
			// Only can commit logs contain current term
			cannotCommitCurrentTerm := false
			for i := rf.getLastApplied()+1; i <= rf.getLogLength(); i++ {
				if rf.getLog(i).Term == args.Term {
					cannotCommitCurrentTerm = true
					break
				}
			}
			// doesn't has current term log entries, so cannot commit
			if !cannotCommitCurrentTerm {
				return
			}
			// set commitIndex = min(leaderCommit, index of last new entry)
			lastEntryIndex := rf.getLog(rf.getLogLength()).Index
			if args.LeaderCommit >= lastEntryIndex {
				rf.setCommitIndex(lastEntryIndex)
			} else {
				rf.setCommitIndex(args.LeaderCommit)
			}
			//DPrintf(" %v start apply %v %v", rf.me, args.LeaderCommit, rf.getAllLog())
			rf.apply()
			rf.persist()
		}
		return
	}
	// following is for replicate log entries
	// log doesn't contain an entry at prevLogIndex
	// whose term matches prevLogTerm
	rf.mu.Lock() // Handle one append without disturb
	defer rf.mu.Unlock()
	reply.ConflictTerm = 0
	logLength := len(rf.log)
	if args.PrevLogIndex > 0 {
		// 没有prevLogIndex
		if logLength < args.PrevLogIndex {
			reply.ConflictIndex = logLength
			reply.ConflictTerm = 0
			reply.Success = false
			reply.Term = rf.currentTerm
			return
		}
		if rf.log[args.PrevLogIndex-1].Term != args.PrevLogTerm {
			reply.ConflictTerm = rf.log[args.PrevLogIndex-1].Term
			reply.ConflictIndex = args.PrevLogIndex
			for i := reply.ConflictIndex-1; i > 0; i-- {
				if rf.log[i-1].Term != reply.ConflictTerm {
					break
				} else {
					reply.ConflictIndex -= 1
				}
			}
			reply.Success = false
			reply.Term = rf.currentTerm
			return
		}
	}
	isMatch := true
	nextIndex := args.PrevLogIndex+1
	for i, entry := range args.Entries {
		if nextIndex + i > logLength {
			isMatch = false
			break
		} else if rf.log[nextIndex+i-1].Term != entry.Term {
			isMatch = false
		}
	}
	if !isMatch {
		// 因为前面已经判断过了，所以这里可以做截断
		entries := make([]LogEntry, len(args.Entries))
		copy(entries, args.Entries)
		rf.log = append(rf.log[:nextIndex-1], entries...)
		//DPrintf("%v append entries %v(%v)", rf.me,
		//	len(rf.log), rf.log[len(rf.log)-1].Index)
	}
	DPrintf("%v receive %v command (%v)", rf.me,
		len(args.Entries), len(rf.log))
	reply.Success = true
	rf.mu.Unlock()
	rf.persist()
	rf.setIsReceivedHeart(true)
	rf.mu.Lock()
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
	rf.persister = persister
	rf.me = me
	// begin as follower
	rf.state = 2
	// wait a span for leader to send
	rf.isReceivedHeart = true
	rf.currentTerm = 0
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.applyChan = applyCh
	for _ = range rf.peers {
		rf.nextIndex = append(rf.nextIndex, 1)
		rf.matchIndex = append(rf.matchIndex, 0)
	}



	// Your initialization code here (2A, 2B, 2C).
	DPrintf("%v init\n", rf.me)
	go rf.electionCheckLoop()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}
