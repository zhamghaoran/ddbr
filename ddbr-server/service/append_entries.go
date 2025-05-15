package service

import (
	"context"
	"encoding/json"
	"zhamghaoran/ddbr-server/infra"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

// AppendEntries 处理AppendEntries RPC请求
func AppendEntries(ctx context.Context, req *sever.AppendEntriesReq) (*sever.AppendEntriesResp, error) {
	resp := &sever.AppendEntriesResp{
		Term:    0,
		Success: false,
	}

	raftState := infra.GetRaftState()

	// 获取当前任期
	currentTerm := raftState.GetCurrentTerm()
	resp.Term = currentTerm

	// 规则1：如果Leader的任期小于当前任期，拒绝请求
	if req.Term < currentTerm {
		log.Log.Infof("Rejecting AppendEntries: leader term %d < current term %d", req.Term, currentTerm)
		return resp, nil
	}

	// 如果Leader任期更大，更新自己的任期并转为Follower
	if req.Term > currentTerm {
		raftState.SetCurrentTerm(req.Term)
		raftState.SetVotedFor(-1)
		resp.Term = req.Term
	}

	// 规则2：如果日志不包含在PrevLogIndex处任期为PrevLogTerm的条目，拒绝请求
	logs := raftState.GetLogs()

	// 检查前一个日志条目
	if req.PrevLogIndex >= 0 {
		// 将基于1的日志索引转换为基于0的切片索引
		sliceIndex := int(req.PrevLogIndex) - 1

		// 如果日志太短，不包含PrevLogIndex
		if req.PrevLogIndex > 0 && sliceIndex >= len(logs) {
			log.Log.CtxInfof(ctx, "Rejecting AppendEntries: log too short, prevLogIndex=%d, logLength=%d",
				req.PrevLogIndex, len(logs))
			return resp, nil
		}

		// 如果日志中的任期与PrevLogTerm不匹配
		if req.PrevLogIndex > 0 && logs[sliceIndex].Term != req.PrevLogTerm {
			log.Log.CtxInfof(ctx, "Rejecting AppendEntries: term mismatch at prevLogIndex=%d, expected=%d, actual=%d",
				req.PrevLogIndex, req.PrevLogTerm, logs[sliceIndex].Term)
			return resp, nil
		}
	}

	// 接受请求，处理日志条目
	resp.Success = true

	// 解析并追加新日志条目
	if len(req.Entries) > 0 {
		log.Log.Infof("Processing %d new log entries from leader", len(req.Entries))

		// 创建新日志条目
		newEntries := make([]*sever.LogEntry, 0, len(req.Entries))
		nextIndex := req.PrevLogIndex + 1
		nextSliceIndex := int(req.PrevLogIndex) // 已经考虑到日志索引从1开始，切片索引从0开始的转换

		for i, entryStr := range req.Entries {
			// 这里应该使用正确的反序列化逻辑
			// 简化处理，直接使用命令字符串创建条目
			logEntry := &sever.LogEntry{
				Term:    req.Term,
				Index:   nextIndex + int64(i),
				Command: entryStr,
			}
			newEntries = append(newEntries, logEntry)
		}

		// 规则3：如果现有日志与Leader冲突，删除并替换
		if nextSliceIndex < len(logs) {
			// 截断日志
			logs = logs[:nextSliceIndex]
		}

		// 追加新日志
		logs = append(logs, newEntries...)
		raftState.SetLogs(logs)

		log.Log.Infof("Updated log with %d entries, new length: %d", len(newEntries), len(logs))
	}

	// 规则4：如果LeaderCommit > CommitIndex，设置CommitIndex = min(LeaderCommit, 最新日志索引)
	if req.LeaderCommit > raftState.GetCommitIndex() {
		// 计算新的CommitIndex
		lastLogIndex := int64(0)
		if len(logs) > 0 {
			lastLogIndex = logs[len(logs)-1].Index
		}

		newCommitIndex := min(req.LeaderCommit, lastLogIndex)
		raftState.SetCommitIndex(newCommitIndex)

		log.Log.Infof("Updated commitIndex to %d", newCommitIndex)

		// 应用已提交但未应用的日志
		if err := raftState.ApplyCommittedLogs(); err != nil {
			log.Log.Errorf("Failed to apply committed logs: %v", err)
		}
	}

	return resp, nil
}

// min 返回两个整数中的较小值
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// 将字符串反序列化为LogEntry
func deserializeLogEntry(data string) (*sever.LogEntry, error) {
	// 简单处理：尝试使用JSON反序列化
	// 实际应用中应当使用与Leader端匹配的序列化方法
	var entry sever.LogEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		// 如果无法反序列化，创建一个简单条目
		return &sever.LogEntry{
			Command: data,
		}, nil
	}
	return &entry, nil
}
