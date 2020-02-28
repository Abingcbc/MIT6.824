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
	"math/rand"
	"sync"
	"time"
)
import "sync/atomic"
import "../labrpc"

// import "bytes"
// import "../labgob"

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

type LogEntry struct {
	EntryIndex int
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
	DPrintf("%v GetState %v %v", rf.me, term, isleader)
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

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
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
	Term 		int
	VoteGranted bool
}

type AppendEntriesArgs struct {
	Term int
	LeaderId int
	PrevLogIndex int
	PrevLogTerm int
}

type AppendEntriesReply struct {
	Term int
	Success bool
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	reply.Term = rf.getCurrentTerm()
	// If server has voted for other server (not self), it cannot vote for a second one
	if rf.getCurrentTerm() < args.Term {
		rf.setCurrentTerm(args.Term)
		rf.setVoteFor(args.CandidateId)
		rf.setState(2)
		reply.VoteGranted = true
		rf.setIsReceivedHeart(true)
		DPrintf("%v vote for %v (%v vs %v)-- %v\n", rf.me,
			args.CandidateId, rf.getCurrentTerm(),
			args.Term, reply.VoteGranted)
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
			positiveVote := 0
			DPrintf("%v start a election term\n", rf.me)
			rf.setCurrentTerm(-1)
			rf.setState(1)
			// vote for self
			rf.setVoteFor(rf.me)
			for i := 0; i < len(rf.peers); i++ {
				if i == rf.me {
					continue
				}
				// if leader has been elected, there is no need to compete
				if rf.getOnlyState() != 1 {
					break
				}
				// avoid bottleneck
				go func(index int) {
					request := RequestVoteArgs{
						Term:         rf.currentTerm,
						CandidateId:  rf.me,
						LastLogIndex: rf.commitIndex,
						LastLogTerm:  rf.currentTerm,
					}
					reply := RequestVoteReply{}
					DPrintf("%v wait for vote from %v", rf.me, index)
					result := rf.sendRequestVote(index, &request, &reply)
					DPrintf("%v get vote from %v", rf.me, index)
					if result && reply.VoteGranted {
						positiveVote++
					} else {
						DPrintf("%v get timeout from %v", rf.me, index)
					}
				}(i)
			}
			// wait for vote, timeout ones will not include
			time.Sleep(100*time.Millisecond)
			// Win majority vote
			// If server still is a candidate, it votes for itself
			if rf.getOnlyState() == 1 && positiveVote >= int(len(rf.peers)/2) {
				DPrintf("%v win the leader\n", rf.me)
				rf.setState(0)
				go rf.heartBeatLoop()
			}
			rf.setIsReceivedHeart(false)
			// Random timeout
			time.Sleep(time.Duration(rand.Intn(150) + 200)*time.Millisecond)
		} else {
			rf.setIsReceivedHeart(false)
			DPrintf("%v wait for heartbeat", rf.me)
			// Election timeout
			time.Sleep(200*time.Millisecond)
		}
	}
}

func (rf *Raft) sendHeartBeat(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.RequestHeartBeat", args, reply)
	return ok
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
					Term:     rf.getCurrentTerm(),
					LeaderId: rf.me,
				}
				reply := AppendEntriesReply{}
				result := rf.sendHeartBeat(index, &args, &reply)
				if result {
					if !reply.Success {
						rf.setState(2)
						rf.setCurrentTerm(reply.Term)
					}
				}
			}(i)
		}
		// period span (10 per second)
		time.Sleep(100*time.Millisecond)
	}
}

// handle heartbeat sent from leader
func (rf *Raft) RequestHeartBeat(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	DPrintf("%v receive heartbeat from %v %v-%v\n", rf.me,
		args.LeaderId, rf.getCurrentTerm(), args.Term)
	reply.Term = rf.getCurrentTerm()
	if args.Term < rf.getCurrentTerm() {
		reply.Success = false
	} else {
		reply.Success = true
		rf.setIsReceivedHeart(true)
		rf.setCurrentTerm(args.Term)
		rf.setState(2)
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
	rf.persister = persister
	rf.me = me
	// begin as follower
	rf.state = 2
	// wait a span for leader to send
	rf.isReceivedHeart = true
	rf.currentTerm = 0

	// Your initialization code here (2A, 2B, 2C).
	DPrintf("%v init\n", rf.me)
	go rf.electionCheckLoop()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}
