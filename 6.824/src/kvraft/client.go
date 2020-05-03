package kvraft

import (
	"../labrpc"
	"sync/atomic"
	"time"
)
import "crypto/rand"
import "math/big"

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	leaderId int
	me       int64
	globalId int64
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.leaderId = 0
	ck.me = nrand()
	ck.globalId = 0
	return ck
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) Get(key string) string {

	// You will have to modify this function.
	args := GetArgs{Key: key}
	var reply GetReply
	DPrintf("[client.Get]: Client %v key %v value %v", ck.me, key, reply.Value)
	for i := 0; i < 8000; i++ {
		reply = GetReply{}
		ok := ck.servers[ck.leaderId].Call("KVServer.Get", &args, &reply)
		if !ok || reply.Err == "NotLeaderError"{
			time.Sleep(time.Millisecond * 10)
			ck.leaderId = (ck.leaderId + 1) % len(ck.servers)
		} else {
			time.Sleep(time.Millisecond * 100)
			return reply.Value
		}
	}
	return ""
}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	args := PutAppendArgs{
		Key:   key,
		Value: value,
		Op:    op,
		ClientId: ck.me,
		MsgId: atomic.AddInt64(&ck.globalId,1),
	}
	DPrintf("[client.PutAppend]: Client %v key %v value %v", ck.me, key, value)
	var reply PutAppendReply
	for i := 0; i < 8000; i++ {
		reply = PutAppendReply{}
		ok := ck.servers[ck.leaderId].Call("KVServer.PutAppend", &args, &reply)
		// some tricky bug may happen if don't wait and print too much logs
		if !ok || reply.Err == "NotLeaderError" {
			time.Sleep(time.Millisecond * 10)
			ck.leaderId = (ck.leaderId + 1) % len(ck.servers)
		} else {
			time.Sleep(time.Millisecond * 100)
			return
		}
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
