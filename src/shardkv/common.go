package shardkv

import (
	"shardmaster"
	"time"
)

//
// Sharded key/value server.
// Lots of replica groups, each running op-at-a-time paxos.
// Shardmaster decides which group serves each shard.
// Shardmaster may change shard assignment from time to time.
//
// You will have to modify these definitions.
//

const (
	OP_GET         = "GET"
	OP_PUT         = "PUT"
	OP_APPEND      = "APPEND"
	OP_RECONFIGURE = "RECONFIGURE"
	CLIENT_TIMEOUT = 1000 * time.Millisecond
)

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongLeader = "ErrWrongLeader"
	ErrWrongGroup  = "ErrWrongGroup"
	ErrNotReady    = "ErrNotReady"
	ErrWrongConfig = "ErrWrongCOnfig"
)

type Err string

// Put or Append
type PutAppendArgs struct {
	// You'll have to add definitions here.
	Key   string
	Value string
	Op    string // "Put" or "Append"
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.

	ClientId int64
	ReqId    int
}

type PutAppendReply struct {
	WrongLeader bool
	Err         Err
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here.

	ClientId int64
	ReqId    int
}

type GetReply struct {
	WrongLeader bool
	Err         Err
	Value       string
}

// to get shards from origin group
type TransferArgs struct {
	ConfigNum int
	Shards    []int
}

type TransferReply struct {
	Shards      [shardmaster.NShards]map[string]string
	Ack         map[int64]int
	WrongLeader bool
	Err         Err
}

type ReconfigureArgs struct {
	Cfg    shardmaster.Config
	Shards [shardmaster.NShards]map[string]string
	Ack    map[int64]int
}

type ReconfigureReply struct {
	Err Err
}
