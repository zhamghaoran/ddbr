package service

import (
	"context"
	"zhamghaoran/ddbr-server/configs"
	"zhamghaoran/ddbr-server/infra"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/common"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

func RequestVote(ctx context.Context, req *sever.RequestVoteReq) (resp *sever.RequestVoteResp, err error) {
	resp = sever.NewRequestVoteResp()
	resp.Common = &common.Common{
		RespCode: 0,
		Message:  "success",
	}

	// 获取Raft状态
	raftState := infra.GetRaftState()

	// 加锁保护并发访问
	raftState.Mu.Lock()
	defer raftState.Mu.Unlock()

	// 1. 如果请求中的任期小于当前任期，拒绝投票
	if req.Term < raftState.CurrentTerm {
		resp.Term = raftState.CurrentTerm
		resp.VoteGranted = false
		resp.Common.Message = "term is smaller than current term"
		return resp, nil
	}

	// 2. 如果请求中的任期大于当前任期，更新当前任期，重置投票状态
	if req.Term > raftState.CurrentTerm {
		raftState.CurrentTerm = req.Term
		raftState.VotedFor = -1 // 重置投票状态
	}

	// 设置响应的任期为当前任期
	resp.Term = raftState.CurrentTerm

	// 3. 检查是否已经投票给其他候选人
	if raftState.VotedFor != -1 && raftState.VotedFor != req.CandidateId {
		resp.VoteGranted = false
		resp.Common.Message = "already voted for another candidate"
		return resp, nil
	}

	// 4. 检查日志完整性
	// 获取本节点的最后日志条目
	var lastLogTerm int64 = 0
	var lastLogIndex int64 = 0
	if len(raftState.Logs) > 0 {
		lastLog := raftState.Logs[len(raftState.Logs)-1]
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
	raftState.VotedFor = req.CandidateId
	resp.VoteGranted = true

	// 6. 持久化状态（这里应该调用持久化方法，这里简化处理）
	// TODO: persistRaftState()

	return resp, nil
}

// SyncLogs 处理从节点的日志同步请求 - 提供给gRPC服务调用
func SyncLogs(ctx context.Context, req *sever.LogSyncReq) (resp *sever.LogSyncResp, err error) {
	log.Log.Infof("Received log sync request from node %d", req.NodeId)

	resp = sever.NewLogSyncResp()

	// 如果自己不是Leader，返回错误
	if !isLeader() {
		resp.Success = false
		resp.Message = "not leader"
		leaderId := int64(-1) // 如果知道当前Leader，设置为Leader的ID
		resp.LeaderId = leaderId
		return resp, nil
	}

	// 获取本地日志
	raftState := infra.GetRaftState()
	logs := raftState.GetLogs()

	// 找出需要同步的日志条目
	var entriesToSync []*sever.LogEntry
	for _, entry := range logs {
		if entry.Index > req.LastLogIndex {
			// 转换日志格式
			syncEntry := &sever.LogEntry{
				Term:    entry.Term,
				Index:   entry.Index,
				Command: entry.Command,
			}
			entriesToSync = append(entriesToSync, syncEntry)
		}
	}

	resp.Success = true
	resp.SetEntries(entriesToSync)
	resp.Message = "sync success"

	log.Log.Infof("Sending %d log entries to node %d", len(entriesToSync), req.NodeId)
	return resp, nil
}

// isLeader 判断当前节点是否是Leader
func isLeader() bool {
	// 使用配置中的IsMaster标志
	return configs.IsMaster()
}

// applyLogToStateMachine 将日志应用到状态机
func applyLogToStateMachine(entry sever.LogEntry) {
	// TODO: 实际实现状态机应用逻辑
	log.Log.Infof("Applied log entry: index=%d, term=%d, command=%s",
		entry.Index, entry.Term, entry.Command)
}
