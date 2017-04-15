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
	"labrpc"
	"math/rand"
	"sync"
	"time"
)

// import "bytes"
// import "encoding/gob"

type RaftRole int

const (
	_              RaftRole = iota
	RAFT_LEADER
	RAFT_CANDIDATE
	RAFT_FOLLOWER
)

const (
	ElectionTimeoutMin   = 550
	ElectionTimeOutRange = 333
	HeartBeatInterval    = 50 * time.Millisecond
)

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
//
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

//
// The Log records one operation
//
type LogEntry struct {
	LogIndex int
	LogTerm  int
	Command  interface{}
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	role        RaftRole
	currentTerm int
	votedFor    int
	voteCount   int
	log         []LogEntry

	commitIndex int
	lastApplied int

	nextIndex  []int
	matchIndex []int

	voteResultChan chan bool
	voteGrantChan  chan bool
	heartBeatChan  chan bool
	commitChan     chan bool
	applyChan      chan ApplyMsg

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int
	CandidateID  int
	LastLogIndex int
	LastLogTerm  int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

// AppendEntriesArgs is example AppendEntriesArgs RPC arguments structure
type AppendEntriesArgs struct {
	Term         int
	LeaderID     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
	NextIndex int
}

// GetState return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	return rf.currentTerm, rf.role == RAFT_LEADER
}

// GetLastLog return the last log
func (rf *Raft) GetLastLog() LogEntry {
	return rf.log[len(rf.log)-1]
}

// GetLastIndex return the last log index
func (rf *Raft) GetLastIndex() int {
	return rf.GetLastLog().LogIndex
}

// GetLastTerm return the term of last log
func (rf *Raft) GetLastTerm() int {
	return rf.GetLastLog().LogTerm
}

func (rf *Raft) GetPrevLog(server int) LogEntry {
	index := rf.nextIndex[server] - 1
	if index >= 0 {
		return rf.log[index]
	} else {
		return LogEntry{LogIndex: -1, LogTerm: 0}
	}
}

func (rf *Raft) PrevLogTerm(server int) int {
	return rf.GetPrevLog(server).LogTerm
}

func (rf *Raft) PrevLogIndex(server int) int {
	return rf.GetPrevLog(server).LogIndex
}

func (rf *Raft) SwitchTo(role RaftRole) {

	rf.role = role
	switch role {
	case RAFT_FOLLOWER:
		rf.voteCount = 0
		rf.votedFor = -1
	case RAFT_CANDIDATE:
		rf.currentTerm++
		rf.votedFor = rf.me
		rf.voteCount = 1
	case RAFT_LEADER:
		rf.nextIndex = make([]int, len(rf.peers))
		rf.matchIndex = make([]int, len(rf.peers))
		for i := range rf.peers {
			rf.nextIndex[i] = rf.GetLastIndex() + 1
			rf.matchIndex[i] = 0
		}
	}
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
	// e := gob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := gob.NewDecoder(r)
	// d.Decode(&rf.xxx)
	// d.Decode(&rf.yyy)
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).

	rf.mu.Lock()
	defer rf.mu.Unlock()

	//DPrintf("server %d at term %d recive vote from server %d at term %d \n", rf.me, rf.currentTerm, args.CandidateID, args.Term)

	reply.VoteGranted = false
	reply.Term = rf.currentTerm

	if args.CandidateID == rf.me {
		return
	}

	if args.Term < rf.currentTerm {
		return
	} else if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.SwitchTo(RAFT_FOLLOWER)
		reply.Term = rf.currentTerm
	}

	if rf.votedFor == -1 || rf.votedFor == args.CandidateID {
		if args.LastLogTerm < rf.GetLastTerm() {
			return
		}

		if args.LastLogTerm == rf.GetLastTerm() && args.LastLogIndex < rf.GetLastIndex() {
			return
		}

		//DPrintf("server %d at term %d vote for server %d at term %d \n", rf.me, rf.currentTerm, args.CandidateID, args.Term)

		reply.VoteGranted = true
		rf.role = RAFT_FOLLOWER
		rf.votedFor = args.CandidateID
		rf.voteCount = 0
		rf.voteGrantChan <- true
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
	DPrintf("server %d at term %d send a vote request to server %d",rf.me,rf.currentTerm,server)
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	if !ok {
		return ok
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.role != RAFT_CANDIDATE || rf.currentTerm != args.Term {

	} else if rf.currentTerm < reply.Term {
		rf.currentTerm = reply.Term
		rf.SwitchTo(RAFT_FOLLOWER)
	} else if reply.VoteGranted {
		rf.voteCount++
		//DPrintf("server %d at term %d get votes %d \n", rf.me, rf.currentTerm, rf.voteCount)
		if rf.voteCount > len(rf.peers)>>1 {
			rf.role = RAFT_LEADER // this is for only one true will get in the voteResult channel
			rf.voteResultChan <- true
		}
	}
	return ok
}

func (rf *Raft) BuildHeartBeat() AppendEntriesArgs {
	var args AppendEntriesArgs
	args.Term = rf.currentTerm
	args.Entries = make([]LogEntry, 0)
	args.LeaderID = rf.me
	args.LeaderCommit = rf.commitIndex
	return args
}

func (rf *Raft) UpdateCommitIndex() int {

	res := 0

	for i := rf.GetLastIndex(); i > rf.commitIndex; i-- {
		if rf.log[i].LogTerm != rf.currentTerm {
			break
		}
		count := 0
		for j:= range rf.peers {
			if rf.matchIndex[j] >= i {
				count++
			}
		}

		if 2*count > len(rf.peers) {
			DPrintf("leader %d at term %d commit index from %d to %d",rf.me,rf.currentTerm,rf.commitIndex,i)
			res = i - rf.commitIndex
			rf.commitIndex = i
			return res
		}
	}

	return res

}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	DPrintf("server %d at term %d recive heartbeat from server %d at term %d \n",rf.me,rf.currentTerm,args.LeaderID,args.Term)

	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm
	reply.Success = false

	if args.Term < rf.currentTerm {
		return
	}

	if args.LeaderID != rf.me {
		rf.heartBeatChan <- true
	}

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.SwitchTo(RAFT_FOLLOWER)
		reply.Term = rf.currentTerm
	}

	if args.PrevLogIndex < 0 {
		args.PrevLogIndex = -1
	} else if rf.GetLastIndex() < args.PrevLogIndex {
		reply.NextIndex = rf.GetLastIndex() + 1
		return
	} else if rf.log[args.PrevLogIndex].LogTerm != args.PrevLogTerm {
		for i:= args.PrevLogIndex - 1; i>= 0; i-- {
			if rf.log[i].LogTerm == args.PrevLogTerm {
				reply.NextIndex = i + 1
				break
			}
		}
		return
	}


	rf.log = append(rf.log[:args.PrevLogIndex+1], args.Entries...)
	reply.Success = true
	reply.NextIndex = rf.GetLastIndex() + 1

	if args.LeaderCommit > rf.commitIndex {
		if args.LeaderCommit < rf.GetLastIndex() {
			rf.commitIndex = args.LeaderCommit
		} else {
			rf.commitIndex = rf.GetLastIndex()
		}
		rf.commitChan <- true
	}
}

func (rf *Raft) SendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	if !ok {
		return ok
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.role != RAFT_LEADER || rf.currentTerm != args.Term {
		return ok
	}

	if rf.currentTerm < reply.Term {
		rf.currentTerm = reply.Term
		rf.SwitchTo(RAFT_FOLLOWER)
		return ok
	}

	if reply.Success {
		if len(args.Entries) > 0 {
			rf.nextIndex[server] = args.Entries[len(args.Entries)-1].LogIndex + 1
			rf.matchIndex[server] = rf.nextIndex[server] - 1
			DPrintf("leader %d at term %d update server %d next index to %d",rf.me,rf.currentTerm,server,rf.nextIndex[server])
		}
	} else {
		rf.nextIndex[server] = reply.NextIndex
		DPrintf("leader %d receive next index %d from server %d",rf.me,reply.NextIndex,server)
	}

	return ok
}

func (rf *Raft) BroadcastAppendEntries() {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	commits := rf.UpdateCommitIndex()
	if commits >0 {
		rf.commitChan <- true
	}

	entry := rf.BuildHeartBeat()

	for i := range rf.peers {
		args := entry
		args.PrevLogIndex = rf.PrevLogIndex(i)
		args.PrevLogTerm = rf.PrevLogTerm(i)
		args.Entries = make([]LogEntry, len(rf.log[args.PrevLogIndex+1:]))
		copy(args.Entries, rf.log[args.PrevLogIndex+1:])
		go func(server int, arg *AppendEntriesArgs) {
			var reply AppendEntriesReply
			rf.SendAppendEntries(server, arg, &reply)
		}(i, &args)
	}
}

func (rf *Raft) BuildRequestVote() RequestVoteArgs {
	var args RequestVoteArgs
	args.CandidateID = rf.me
	args.Term = rf.currentTerm
	args.LastLogIndex = rf.GetLastIndex()
	args.LastLogTerm = rf.GetLastTerm()
	return args
}

// BroadcastRequestVotes sends reauestVote rpc to all peers it knows
func (rf *Raft) BroadcastRequestVotes() {

	rf.mu.Lock()
	//DPrintf("server %d on term %d request a vote! \n", rf.me, rf.currentTerm)
	args := rf.BuildRequestVote()
	rf.mu.Unlock()

	for i := range rf.peers {
		if rf.role != RAFT_CANDIDATE {break}
		go func(index int) {
			var reply RequestVoteReply
			rf.sendRequestVote(index, &args, &reply)
		}(i)
	}
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {

	// Your code here (2B).

	rf.mu.Lock()
	defer rf.mu.Unlock()
	index := -1
	term := rf.currentTerm
	isLeader := rf.role == RAFT_LEADER

	if isLeader {
		index = rf.GetLastIndex() + 1
		rf.log = append(rf.log, LogEntry{LogTerm: term, LogIndex: index, Command: command})

		DPrintf("client request log {index:%d} to leader %d at term %d",index,rf.me,rf.currentTerm)
	}

	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

func (rf *Raft) Loop() {
	for {
		switch rf.role {
		case RAFT_LEADER:
			rf.BroadcastAppendEntries()
			time.Sleep(HeartBeatInterval)
		case RAFT_FOLLOWER:
			select {
			case <-rf.heartBeatChan:
			case <-rf.voteGrantChan:
			case <-time.After(time.Duration(ElectionTimeoutMin+rand.Int63n(ElectionTimeOutRange)) * time.Millisecond):
				rf.mu.Lock()
				rf.role = RAFT_CANDIDATE
				rf.mu.Unlock()
			}
		case RAFT_CANDIDATE:
			rf.mu.Lock()
			rf.SwitchTo(RAFT_CANDIDATE)
			rf.mu.Unlock()
			go rf.BroadcastRequestVotes()

			select {
			case win := <-rf.voteResultChan:
				if win {
					rf.mu.Lock()
					rf.SwitchTo(RAFT_LEADER)
					rf.mu.Unlock()
					DPrintf("server %d at term %d win the vote \n", rf.me, rf.currentTerm)
				}
			case <-rf.heartBeatChan:
			case <-rf.voteGrantChan:
			case <-time.After(time.Duration(ElectionTimeoutMin+rand.Int63n(ElectionTimeOutRange)) * time.Millisecond):
			}
		}
	}
}

func (rf *Raft) CommitLoop() {
	for {
		select {
		case <-rf.commitChan:
			rf.mu.Lock()
			for rf.lastApplied < rf.commitIndex {
				rf.lastApplied++
				msg := ApplyMsg{Index:rf.lastApplied,Command:rf.log[rf.lastApplied].Command}
				DPrintf("server %d at term %d apply message of index %d",rf.me,rf.currentTerm,msg.Index)
				rf.applyChan <- msg
			}
			rf.mu.Unlock()
		}
	}
}

func (rf *Raft) Init(applyCh chan ApplyMsg) {
	rf.role = RAFT_FOLLOWER
	rf.votedFor = -1
	rf.currentTerm = 0
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.log = append(rf.log, LogEntry{LogTerm: 0,LogIndex:0})

	rf.heartBeatChan = make(chan bool, 100)
	rf.voteGrantChan = make(chan bool, 100)
	rf.voteResultChan = make(chan bool, 100)
	rf.commitChan = make(chan bool,100)
	rf.applyChan = applyCh
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

	// Your initialization code here (2A, 2B, 2C).

	rf.Init(applyCh)

	go rf.Loop()

	go rf.CommitLoop()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}
