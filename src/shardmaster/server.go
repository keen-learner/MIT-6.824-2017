package shardmaster

import "raft"
import "labrpc"
import "sync"
import (
	"encoding/gob"
	"time"
	"runtime/debug"
	"os"
	"log"
)

const Debug = 0

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

func DPrintln(a ...interface{}) {
	if Debug > 0 {
		log.Println(a...)
	}
	return
}

const (
	OP_JOIN  = "JOIN"
	OP_LEAVE = "LEAVE"
	OP_MOVE  = "MOVE"
	OP_QUERY = "QUERY"
)

type ShardMaster struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	// Your data here.

	current int      // the current config number
	configs []Config // indexed by config num
	results map[int]chan Result
	ack     map[int64]int
}

type Op struct {
	// Your data here.
	Type string
	Args interface{}
}

type Result struct {
	Type  string
	Args  interface{}
	Reply interface{}
}

func (sm *ShardMaster) WaitForApply(index int) (Result, bool) {
	sm.mu.Lock()
	if _, ok := sm.results[index]; !ok {
		sm.results[index] = make(chan Result, 1)
	}
	resultChan := sm.results[index]
	sm.mu.Unlock()

	select {
	case result := <-resultChan:
		return result, true
	case <-time.After(time.Second * 1):
		return Result{}, false
	}
}

func (sm *ShardMaster) Join(args *JoinArgs, reply *JoinReply) {
	op := Op{Type: OP_JOIN, Args: *args}
	index, _, isLeader := sm.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return
	}

	msg, ok := sm.WaitForApply(index)
	if !ok {
		reply.WrongLeader = true
		return
	}

	if recArgs, ok := msg.Args.(JoinArgs); !ok {
		reply.WrongLeader = true
	} else if args.ClientId != recArgs.ClientId || args.ReqId != recArgs.ReqId {
		reply.WrongLeader = true
	} else {
		reply.Err = msg.Reply.(JoinReply).Err
		reply.WrongLeader = msg.Reply.(JoinReply).WrongLeader
	}
}

func (sm *ShardMaster) Leave(args *LeaveArgs, reply *LeaveReply) {
	op := Op{Type: OP_LEAVE, Args: *args}
	index, _, isLeader := sm.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return
	}

	msg, ok := sm.WaitForApply(index)
	if !ok {
		reply.WrongLeader = true
		return
	}

	if recArgs, ok := msg.Args.(LeaveArgs); !ok {
		reply.WrongLeader = true
	} else if args.ClientId != recArgs.ClientId || args.ReqId != recArgs.ReqId {
		reply.WrongLeader = true
	} else {
		reply.Err = msg.Reply.(LeaveReply).Err
		reply.WrongLeader = msg.Reply.(LeaveReply).WrongLeader
	}

}

func (sm *ShardMaster) Move(args *MoveArgs, reply *MoveReply) {
	op := Op{Type: OP_MOVE, Args: *args}
	index, _, isLeader := sm.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return
	}

	msg, ok := sm.WaitForApply(index)
	if !ok {
		reply.WrongLeader = true
		return
	}

	if recArgs, ok := msg.Args.(MoveArgs); !ok {
		reply.WrongLeader = true
	} else if args.ClientId != recArgs.ClientId || args.ReqId != recArgs.ReqId {
		reply.WrongLeader = true
	} else {
		reply.Err = msg.Reply.(MoveReply).Err
		reply.WrongLeader = msg.Reply.(MoveReply).WrongLeader
	}

}

func (sm *ShardMaster) Query(args *QueryArgs, reply *QueryReply) {
	op := Op{Type: OP_QUERY, Args: *args}
	index, _, isLeader := sm.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		return
	}

	msg, ok := sm.WaitForApply(index)
	if !ok {
		reply.WrongLeader = true
		return
	}

	if recArgs, ok := msg.Args.(QueryArgs); !ok {
		reply.WrongLeader = true
	} else if args.ClientId != recArgs.ClientId || args.ReqId != recArgs.ReqId {
		reply.WrongLeader = true
	} else {
		*reply = msg.Reply.(QueryReply)
		reply.WrongLeader = false
	}

}

//
// the tester calls Kill() when a ShardMaster instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (sm *ShardMaster) Kill() {
	sm.rf.Kill()
	// Your code here, if desired.
}

// needed by shardkv tester
func (sm *ShardMaster) Raft() *raft.Raft {
	return sm.rf
}

func (sm *ShardMaster) Init() {
	sm.current = 0
	sm.ack = make(map[int64]int)
	sm.results = make(map[int]chan Result,1)
}

func (sm *ShardMaster) IsDuplicated(clientId int64, reqId int) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if last, ok := sm.ack[clientId]; ok && last >= reqId {
		return true
	}

	sm.ack[clientId] = reqId
	return false
}

func (sm *ShardMaster) ApplyOp(op Op, isDuplicated bool) interface{} {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	switch op.Args.(type) {
	case JoinArgs:
		var reply JoinReply
		if !isDuplicated {
			sm.ApplyJoin(op.Args.(JoinArgs))
			DPrintln(sm.me, "apply Join", op.Args.(JoinArgs), "->", sm.configs[sm.current])
		}
		reply.Err = OK
		reply.WrongLeader = false
		return reply
	case LeaveArgs:
		var reply LeaveReply
		if !isDuplicated {
			sm.ApplyLeave(op.Args.(LeaveArgs))
			DPrintln(sm.me, "apply Leave", op.Args.(LeaveArgs), "->", sm.configs[sm.current])
		}
		reply.Err = OK
		return reply
	case MoveArgs:
		var reply MoveReply
		if !isDuplicated {
			sm.ApplyMove(op.Args.(MoveArgs))
		}
		reply.Err = OK
		DPrintln(sm.me, "apply Move", op.Args.(MoveArgs), "->", sm.configs[sm.current])
		return reply
	case QueryArgs:
		var reply QueryReply
		args := op.Args.(QueryArgs)
		if args.Num == -1 || args.Num > sm.current {
			reply.Config = sm.configs[sm.current]
		} else {
			reply.Config = sm.configs[args.Num]
		}
		reply.Err = OK
		DPrintln(sm.me, "apply Query", op.Args.(QueryArgs), "->", sm.configs[sm.current])
		return reply
	}
	return nil
}

func (sm *ShardMaster) ApplyJoin(args JoinArgs) {
	cfg := sm.NextConfig()
	needRebalance := false
	gids := make([]int, 0)
	for gid, servers := range args.Servers {
		if _, exists := cfg.Groups[gid]; !exists {
			cfg.Groups[gid] = servers
			needRebalance = true
			gids = append(gids, gid)
		}
	}

	if needRebalance {
		DPrintln("need rebalance!")
		sm.Rebalance(cfg, OP_JOIN, gids)
	}
}

func (sm *ShardMaster) ApplyLeave(args LeaveArgs) {
	cfg := sm.NextConfig()
	needRebalance := false
	gids := make([]int, 0)

	for _, gid := range args.GIDs {
		if _, exists := cfg.Groups[gid]; exists {
			needRebalance = true
			gids = append(gids, gid)
		}
	}
	if needRebalance {
		sm.Rebalance(cfg, OP_LEAVE, gids)
	}

	for _, gid := range gids {
		delete(cfg.Groups, gid)
	}
}

func (sm *ShardMaster) ApplyMove(args MoveArgs) {
	cfg := sm.NextConfig()
	cfg.Shards[args.Shard] = args.GID
}

func (sm *ShardMaster) NextConfig() *Config {
	var c Config
	c.Num = sm.current + 1
	c.Shards = sm.configs[sm.current].Shards
	c.Groups = map[int][]string{}
	for k, v := range sm.configs[sm.current].Groups {
		c.Groups[k] = v
	}
	sm.current += 1
	sm.configs = append(sm.configs, c)
	return &sm.configs[sm.current]
}

func (sm *ShardMaster) GetMinGidByShards(shardsCount map[int][]int) int {
	min := -1
	var gid int
	for k, v := range shardsCount {
		if min == -1 || min > len(v) {
			min = len(v)
			gid = k
		}
	}
	return gid
}

func (sm *ShardMaster) GetMaxGidByShards(shardsCount map[int][]int) int {
	max := -1
	var gid int
	for k, v := range shardsCount {
		if max < len(v) {
			max = len(v)
			gid = k
		}
	}
	return gid
}

func (sm *ShardMaster) CountShards(cfg *Config) map[int][]int {
	shardsCount := map[int][]int{}
	for k := range cfg.Groups {
		shardsCount[k] = []int{}
	}
	for k, v := range cfg.Shards {
		shardsCount[v] = append(shardsCount[v], k)
	}
	return shardsCount
}

func (sm *ShardMaster) Rebalance(cfg *Config, opType string, gids []int) {
	counts := sm.CountShards(cfg)
	switch opType {
	case OP_JOIN:
		meanNum := NShards / len(cfg.Groups)
		for _,gid := range gids {
			for i:=0; i < meanNum; i++ {
				maxGid := sm.GetMaxGidByShards(counts)
				if len(counts[maxGid]) == 0 {
					DPrintf("ReBalanceShards: max gid does not have shards")
					debug.PrintStack()
					os.Exit(-1)
				}
				cfg.Shards[counts[maxGid][0]] = gid
				counts[maxGid] = counts[maxGid][1:]
			}
		}

		if cfg.Num == 1 {
			for i := 0; i < NShards; i++ {
				if cfg.Shards[i] == 0 {
					minGid := sm.GetMinGidByShards(counts)
					cfg.Shards[i] = minGid
					counts[minGid] = append(counts[minGid],i)
				}
			}
		}
	case OP_LEAVE:
		shards := make([]int, 0)
		for _, gid := range gids {
			shards = append(shards, counts[gid]...)
			delete(counts, gid)
		}

		for _, shard := range shards {
			minGid := sm.GetMinGidByShards(counts)
			cfg.Shards[shard] = minGid
			counts[minGid] = append(counts[minGid], shard)
		}
	}
}

func (sm *ShardMaster) SendResult(index int, result Result) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.results[index]; !ok {
		sm.results[index] = make(chan Result, 1)
	} else {
		select {
		case <-sm.results[index]:
		default:
		}
	}

	sm.results[index] <- result
}

func (sm *ShardMaster) Loop() {
	for {
		msg := <-sm.applyCh
		req := msg.Command.(Op)

		var result Result
		var clientId int64
		var reqId int

		switch req.Type {
		case OP_JOIN:
			args := req.Args.(JoinArgs)
			clientId = args.ClientId
			reqId = args.ReqId
			result.Args = args
		case OP_LEAVE:
			args := req.Args.(LeaveArgs)
			clientId = args.ClientId
			reqId = args.ReqId
			result.Args = args
		case OP_MOVE:
			args := req.Args.(MoveArgs)
			clientId = args.ClientId
			reqId = args.ReqId
			result.Args = args
		case OP_QUERY:
			args := req.Args.(QueryArgs)
			clientId = args.ClientId
			reqId = args.ReqId
			result.Args = args
		}

		result.Type = req.Type
		result.Reply = sm.ApplyOp(req, sm.IsDuplicated(clientId, reqId))
		sm.SendResult(msg.Index, result)
		sm.CheckConfigValid()
	}
}

func (sm *ShardMaster) CheckConfigValid() {
	c := sm.configs[sm.current]
	for _, v := range c.Shards {
		//!!! init group is zero
		if len(c.Groups) == 0 && v == 0 {
			continue
		}
		if _, ok := c.Groups[v]; !ok {
			DPrintln("Check failed that", v, "group does not exit", c.Shards, c.Groups)
			debug.PrintStack()
			os.Exit(-1)
		}
	}
}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Paxos to
// form the fault-tolerant shardmaster service.
// me is the index of the current server in servers[].
//
func StartServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister) *ShardMaster {
	sm := new(ShardMaster)
	sm.me = me

	sm.configs = make([]Config, 1)
	sm.configs[0].Groups = map[int][]string{}

	gob.Register(Op{})
	gob.Register(JoinArgs{})
	gob.Register(LeaveArgs{})
	gob.Register(MoveArgs{})
	gob.Register(QueryArgs{})
	gob.Register(JoinReply{})
	gob.Register(LeaveReply{})
	gob.Register(MoveReply{})
	gob.Register(QueryReply{})

	sm.applyCh = make(chan raft.ApplyMsg)
	sm.rf = raft.Make(servers, me, persister, sm.applyCh)

	sm.Init()

	go sm.Loop()

	// Your code here.

	return sm
}
