package infra

import (
	"context"
	"sync"
	"time"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

// ReplicateLogs 复制日志到所有Follower节点
// 返回已成功复制到多数节点的日志索引
func ReplicateLogs(ctx context.Context, newEntry *sever.LogEntry) (int64, error) {
	raftState := GetRaftState()

	// 非Leader不应该复制日志
	if !raftState.IsMaster {
		return -1, nil
	}

	// 如果没有Follower节点，自己就构成多数派
	if len(raftState.Peers) == 0 {
		// 更新CommitIndex（单节点情况下直接提交）
		newCommitIndex := newEntry.Index
		raftState.SetCommitIndex(newCommitIndex)

		// 应用已提交的日志
		if err := raftState.ApplyCommittedLogs(); err != nil {
			log.Log.Errorf("Failed to apply committed logs: %v", err)
			return -1, err
		}

		return newCommitIndex, nil
	}

	// 初始化复制计数器
	replicationCount := 1                          // 包括自己
	successThreshold := len(raftState.Peers)/2 + 1 // 需要多数节点成功

	// 创建等待组
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 向每个Follower发送AppendEntries请求
	for _, peer := range raftState.Peers {
		wg.Add(1)
		go func(peerAddr string) {
			defer wg.Done()

			// 获取NextIndex
			raftState.Mu.Lock()
			nextIdx, ok := raftState.NextIndex[peerAddr]
			if !ok {
				// 如果不存在，初始化为日志的最后一个索引+1
				nextIdx = int64(len(raftState.Logs))
				raftState.NextIndex[peerAddr] = nextIdx
			}
			currentTerm := raftState.CurrentTerm
			raftState.Mu.Unlock()

			// 准备发送的日志条目
			var entriesToSend []*sever.LogEntry

			// 获取需要发送的日志条目
			raftState.Mu.Lock()
			if nextIdx <= newEntry.Index {
				for i := nextIdx; i <= newEntry.Index && int(i) < len(raftState.Logs); i++ {
					entriesToSend = append(entriesToSend, raftState.Logs[i])
				}
			}

			// 获取前一个日志的索引和任期
			prevLogIndex := nextIdx - 1
			var prevLogTerm int64 = 0
			if prevLogIndex >= 0 && int(prevLogIndex) < len(raftState.Logs) {
				prevLogTerm = raftState.Logs[prevLogIndex].Term
			}

			// 获取当前的commitIndex
			commitIndex := raftState.CommitIndex
			raftState.Mu.Unlock()

			// 如果没有日志需要发送，返回
			if len(entriesToSend) == 0 {
				return
			}

			// 创建AppendEntries请求
			req := &sever.AppendEntriesReq{
				Term:         currentTerm,
				LeaderId:     raftState.GetNodeId(),
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      convertLogEntriesToStrings(entriesToSend),
				LeaderCommit: commitIndex,
			}

			// 发送AppendEntries请求
			leaderClient := GetLeaderClient(peerAddr)
			if leaderClient == nil {
				log.Log.Errorf("Failed to get client for peer %s", peerAddr)
				return
			}

			// 使用带超时的上下文
			timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			log.Log.Infof("Sending AppendEntries to %s with %d entries", peerAddr, len(entriesToSend))

			resp, err := leaderClient.AppendEntries(timeoutCtx, req)
			if err != nil {
				log.Log.Errorf("Failed to send AppendEntries to %s: %v", peerAddr, err)
				return
			}

			// 处理响应
			if resp.Success {
				// 更新MatchIndex和NextIndex
				raftState.Mu.Lock()
				raftState.MatchIndex[peerAddr] = newEntry.Index
				raftState.NextIndex[peerAddr] = newEntry.Index + 1
				raftState.Mu.Unlock()

				// 增加复制计数
				mu.Lock()
				replicationCount++
				mu.Unlock()

				log.Log.Infof("Successfully replicated log to %s, new matchIndex: %d", peerAddr, newEntry.Index)
			} else {
				// 如果失败，可能是日志不一致，减少NextIndex重试
				raftState.Mu.Lock()
				if resp.Term > raftState.CurrentTerm {
					// 如果收到更高的任期，转为Follower
					raftState.CurrentTerm = resp.Term
					raftState.VotedFor = -1
					raftState.IsMaster = false
					log.Log.Warnf("Discovered higher term from %s, stepping down from leader", peerAddr)
				} else {
					// 减少NextIndex并稍后重试
					raftState.NextIndex[peerAddr] = max(1, raftState.NextIndex[peerAddr]-1)
					log.Log.Infof("AppendEntries rejected by %s, reducing nextIndex to %d",
						peerAddr, raftState.NextIndex[peerAddr])
				}
				raftState.Mu.Unlock()
			}
		}(peer)
	}

	// 等待所有复制操作完成或超时
	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	// 使用超时机制
	select {
	case <-wgDone:
		// 所有复制操作已完成
	case <-time.After(3 * time.Second):
		log.Log.Warn("Replication timed out waiting for all followers")
	}

	log.Log.Infof("Replication completed with %d/%d successful replications", replicationCount, len(raftState.Peers)+1)

	// 检查是否达到多数派
	if replicationCount >= successThreshold {
		// 更新CommitIndex
		raftState.UpdateCommitIndex()

		// 应用已提交的日志
		if err := raftState.ApplyCommittedLogs(); err != nil {
			log.Log.Errorf("Failed to apply committed logs: %v", err)
			return -1, err
		}

		return raftState.GetCommitIndex(), nil
	}

	return -1, nil
}

// convertLogEntriesToStrings 将LogEntry数组转换为字符串数组
// Thrift要求AppendEntriesReq中的entries是字符串数组
func convertLogEntriesToStrings(entries []*sever.LogEntry) []string {
	// 实际实现应该使用JSON或其他序列化方式
	// 这里简化处理，只取Command字段
	result := make([]string, len(entries))
	for i, entry := range entries {
		result[i] = entry.Command
	}
	return result
}

// max 返回两个整数中的较大值
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
