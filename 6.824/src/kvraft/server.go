package kvraft

import (
	"../labgob"
	"../labrpc"
	"../raft"
	"bytes"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const Debug = 0

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Command    interface{}
	NotifyChan chan bool
	OpType     string
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	Storage     map[string]string
	ClientMsgId map[int64]int64
	// for snapshot
	lastLogIndex int
}

func (kv *KVServer) isRepeated(clientId int64, msgId int64, update bool) bool {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	lastMsgId, ok := kv.ClientMsgId[clientId]
	compare := false
	if ok {
		compare = lastMsgId >= msgId
	}
	// not found or bigger
	if update && !compare {
		kv.ClientMsgId[clientId] = msgId
	}
	return compare
}

func (kv *KVServer) applyLoop() {
	for !kv.killed() {
		select {
		case msg := <-kv.applyCh:
			if msg.CommandValid {
				DPrintf("[applyLoop]: %v apply %v", kv.me, msg.CommandIndex)
				kv.apply(msg)
			} else {
				if snapshot, ok := msg.Command.(raft.Snapshot); ok {
					kv.updateFromSnapshot(snapshot.Index, snapshot.Storage)
				}
				kv.ifSnapshot()
			}
		}
	}
}

func (kv *KVServer) apply(msg raft.ApplyMsg) {
	op := msg.Command.(Op)
	if op.OpType != "Get" {
		command := op.Command.(PutAppendArgs)
		if !kv.isRepeated(command.ClientId, command.MsgId, true) {
			kv.mu.Lock()
			if op.OpType == "Put" {
				kv.Storage[command.Key] = command.Value
			} else if op.OpType == "Append" {
				kv.Storage[command.Key] += command.Value
			}
			if len(kv.Storage[command.Key]) < 100 {
				DPrintf("[server.apply]: %v CId %v MsgId %v "+
					"%v key %v value %v", kv.me, command.ClientId, command.MsgId,
					op.OpType, command.Key, kv.Storage[command.Key])
			} else {
				DPrintf("[server.apply]: %v CId %v MsgId %v "+
					"%v key %v", kv.me, command.ClientId, command.MsgId,
					op.OpType, command.Key)
			}
			kv.mu.Unlock()
		}
	} else {
		command := op.Command.(GetArgs)
		DPrintf("[server.apply]: %v Get key %v", kv.me, command.Key)
	}
	select {
	case op.NotifyChan <- true:
	default:
	}
	kv.ifSnapshot()
}

func (kv *KVServer) requestRaft(clientId int64, msgId int64,
	args interface{}, opType string) bool {
	if msgId > 0 && kv.isRepeated(clientId, msgId, false) {
		return true
	}
	op := Op{
		Command:    args,
		NotifyChan: make(chan bool),
		OpType:     opType,
	}
	index, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		return false
	}
	DPrintf("[requestRaft]: %v ClientId %v MsgId %v", kv.me, clientId, msgId)
	select {
	case <-op.NotifyChan:
		// update newest index for snapshot
		kv.mu.Lock()
		kv.lastLogIndex = index
		kv.mu.Unlock()
		return true
	case <-time.After(time.Millisecond * 1000):
		return false
	}
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	// get also need to commit
	msg := kv.requestRaft(-1, -1, *args, "Get")
	if msg {
		kv.mu.Lock()
		value, ok := kv.Storage[args.Key]
		kv.mu.Unlock()
		if ok {
			//DPrintf("[server.Get]: key %v", args.Key)
			reply.Value = value
			reply.Err = ""
		} else {
			reply.Value = ""
			reply.Err = "NotFoundError"
		}
	} else {
		reply.Value = ""
		reply.Err = "NotLeaderError"
	}
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	msg := kv.requestRaft(args.ClientId, args.MsgId, *args, args.Op)
	if msg {
		reply.Err = ""
	} else {
		reply.Err = "NotLeaderError"
	}
}

func (kv *KVServer) ifSnapshot() {
	if kv.maxraftstate != -1 && kv.rf.Persister.RaftStateSize() >= kv.maxraftstate {
		writer := new(bytes.Buffer)
		encoder := labgob.NewEncoder(writer)
		encoder.Encode(kv.ClientMsgId)
		encoder.Encode(kv.Storage)
		data := writer.Bytes()
		kv.rf.SaveSnapshot(kv.lastLogIndex, data)
	}
}

// for follower
func (kv *KVServer) updateFromSnapshot(index int, data []byte)  {
	if data == nil || len(data) < 1 {
		return
	}
	// follower should obey leader's index
	kv.mu.Lock()
	kv.lastLogIndex = index
	reader := bytes.NewBuffer(data)
	decoder := labgob.NewDecoder(reader)
	if decoder.Decode(&kv.ClientMsgId) != nil ||
		decoder.Decode(&kv.Storage) != nil  {
		DPrintf("Error in unmarshal raft state")
	}
	log.Printf("[updateFromSnapshot]: Id %v update to %v",
		kv.me, index)
	kv.mu.Unlock()
}

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
//
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
//

func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})
	labgob.Register(PutAppendArgs{})
	labgob.Register(GetArgs{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg, 1000)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	// You may need initialization code here.
	kv.Storage = map[string]string{}
	kv.ClientMsgId = map[int64]int64{}

	go kv.applyLoop()
	return kv
}
