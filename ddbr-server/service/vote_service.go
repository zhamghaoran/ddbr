package service

import (
	"context"
	"sync"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/common"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"

	"github.com/google/uuid"
)

// RaftState 表示Raft节点的状态
type RaftState struct {
	mu          sync.Mutex // 用于保护并发访问
	currentTerm int64      // 当前任期
	votedFor    int64      // 当前任期投票给的候选人ID，如果没有则为-1
	logs        []LogEntry // 日志条目
	nodeId      int64      // 当前节点ID
}

// LogEntry 表示日志条目
type LogEntry struct {
	Term    int64  // 写入该日志时的任期
	Index   int64  // 日志索引
	Command string // 日志命令内容
}

// 全局Raft状态实例
var raftState = &RaftState{
	currentTerm: 0,
	votedFor:    -1,
	logs:        []LogEntry{},
	nodeId:      int64(uuid.New().ID()), // 默认节点ID，实际使用时应该从配置中读取
}

func RequestVote(ctx context.Context, req *sever.RequestVoteReq) (resp *sever.RequestVoteResp, err error) {
	resp = sever.NewRequestVoteResp()
	resp.Common = &common.Common{
		RespCode: 0,
		Message:  "success",
	}

	// 加锁保护并发访问
	raftState.mu.Lock()
	defer raftState.mu.Unlock()

	// 1. 如果请求中的任期小于当前任期，拒绝投票
	if req.Term < raftState.currentTerm {
		resp.Term = raftState.currentTerm
		resp.VoteGranted = false
		resp.Common.Message = "term is smaller than current term"
		return resp, nil
	}

	// 2. 如果请求中的任期大于当前任期，更新当前任期，重置投票状态
	if req.Term > raftState.currentTerm {
		raftState.currentTerm = req.Term
		raftState.votedFor = -1 // 重置投票状态
	}

	// 设置响应的任期为当前任期
	resp.Term = raftState.currentTerm

	// 3. 检查是否已经投票给其他候选人
	if raftState.votedFor != -1 && raftState.votedFor != req.CandidateId {
		resp.VoteGranted = false
		resp.Common.Message = "already voted for another candidate"
		return resp, nil
	}

	// 4. 检查日志完整性
	// 获取本节点的最后日志条目
	var lastLogTerm int64 = 0
	var lastLogIndex int64 = 0
	if len(raftState.logs) > 0 {
		lastLog := raftState.logs[len(raftState.logs)-1]
		lastLogTerm = lastLog.Term
		lastLogIndex = lastLog.Index
	}

	// 如果候选人的日志不够新（任期较小或任期相同但索引较小），拒绝投票
	logOK := (req.LastLogTerm > lastLogTerm) ||
		(req.LastLogTerm == lastLogTerm && req.LastLogIndex >= lastLogIndex)

	if !logOK {
		resp.VoteGranted = false
		resp.Common.Message = "candidate's log is not up-to-date"
		return resp, nil
	}

	// 5. 授予投票
	raftState.votedFor = req.CandidateId
	resp.VoteGranted = true

	// 6. 持久化状态（这里应该调用持久化方法，这里简化处理）
	// TODO: persistRaftState()

	return resp, nil
}
