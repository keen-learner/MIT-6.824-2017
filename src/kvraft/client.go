package raftkv

import "labrpc"
import "crypto/rand"
import (
	"math/big"
	"sync"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.

	sessionId     int64
	reqId         int
	mu            sync.Mutex
	currentLeader int
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

	ck.sessionId = nrand()
	ck.reqId = 0
	return ck
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("RaftKV.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) Get(key string) string {
	// You will have to modify this function.
	var args GetArgs
	args.Key = key
	args.SessionID = ck.sessionId
	var leader int
	var serverNum int
	ck.mu.Lock()
	args.ReqID = ck.reqId
	leader = ck.currentLeader
	serverNum = len(ck.servers)
	ck.reqId++
	ck.mu.Unlock()

	for i := leader; ; i = (i + 1) % serverNum {
		server := ck.servers[i]
		var reply GetReply
		ok := server.Call("RaftKV.Get", &args, &reply)

		if ok && !reply.WrongLeader {
			if i != leader {
				ck.mu.Lock()
				ck.currentLeader = i
				ck.mu.Unlock()
			}
			return reply.Value
		}
	}

}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("RaftKV.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	var args PutAppendArgs
	args.Key = key
	args.Value = value
	args.Op = op
	args.SessionID = ck.sessionId

	ck.mu.Lock()
	args.ReqID = ck.reqId
	ck.reqId++
	ck.mu.Unlock()

	for {
		for _, server := range ck.servers {
			var reply PutAppendReply
			ok := server.Call("RaftKV.PutAppend", &args, &reply)

			if ok && !reply.WrongLeader {
				return
			}
		}
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
