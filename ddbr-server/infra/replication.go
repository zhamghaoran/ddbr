package infra

import (
	"context"
	"sync"
	"time"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

// ReplicateLogs 实现两阶段提交：
// 1. 预提交阶段：将日志复制到多数节点
// 2. 提交阶段：确认多数节点已有日志后，才应用到状态机
// 返回已成功提交的日志索引
func ReplicateLogs(ctx context.Context, newEntry *sever.LogEntry) (int64, error) {
	raftState := GetRaftState()

	// 非Leader不应该复制日志
	if !raftState.IsMaster {
		return -1, nil
	}

	log.Log.Infof("开始两阶段提交过程 - 日志: %+v", newEntry)

	// ====== 阶段1: 预提交 - 将日志复制到多数节点 ======
	log.Log.Infof("阶段1: 预提交 - 将日志复制到节点")

	// 如果没有Follower节点，自己就构成多数派
	if len(raftState.Peers) == 0 {
		log.Log.Infof("单节点环境，跳过复制阶段")
		// 标记为预提交状态
		newEntry.PreCommitted = true

		// 直接进入提交阶段
		return commitLog(ctx, newEntry)
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

			// 创建AppendEntries请求，标记为预提交阶段
			req := &sever.AppendEntriesReq{
				Term:         currentTerm,
				LeaderId:     raftState.GetNodeId(),
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      convertLogEntriesToStrings(entriesToSend),
				LeaderCommit: commitIndex,
				IsPreCommit:  true, // 标记为预提交阶段
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

			log.Log.Infof("发送预提交请求到 %s，共 %d 条日志", peerAddr, len(entriesToSend))

			resp, err := leaderClient.AppendEntries(timeoutCtx, req)
			if err != nil {
				log.Log.Errorf("预提交阶段发送失败 %s: %v", peerAddr, err)
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

				log.Log.Infof("预提交成功，节点: %s，日志索引: %d", peerAddr, newEntry.Index)
			} else {
				// 如果失败，可能是日志不一致，减少NextIndex重试
				raftState.Mu.Lock()
				if resp.Term > raftState.CurrentTerm {
					// 如果收到更高的任期，转为Follower
					raftState.CurrentTerm = resp.Term
					raftState.VotedFor = -1
					raftState.IsMaster = false
					log.Log.Warnf("发现更高任期 %s，降级为follower", peerAddr)
				} else {
					// 减少NextIndex并稍后重试
					raftState.NextIndex[peerAddr] = max(1, raftState.NextIndex[peerAddr]-1)
					log.Log.Infof("预提交被拒绝 %s，减少nextIndex到 %d",
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
		log.Log.Warn("预提交阶段超时")
	}

	log.Log.Infof("预提交阶段完成，成功: %d/%d", replicationCount, len(raftState.Peers)+1)

	// 检查是否达到多数派
	if replicationCount >= successThreshold {
		// 标记为预提交成功
		newEntry.PreCommitted = true
		log.Log.Infof("预提交阶段成功，日志已在多数节点上复制，进入提交阶段")

		// ====== 阶段2: 提交 - 应用到状态机 ======
		return commitLog(ctx, newEntry)
	}

	log.Log.Warnf("预提交阶段失败，未能复制到多数节点")
	return -1, nil
}

// commitLog 实现提交阶段，将预提交的日志真正提交
func commitLog(ctx context.Context, entry *sever.LogEntry) (int64, error) {
	raftState := GetRaftState()

	log.Log.Infof("阶段2: 提交阶段 - 日志索引 %d", entry.Index)

	// 更新CommitIndex
	newCommitIndex := entry.Index
	raftState.SetCommitIndex(newCommitIndex)

	// 发送提交确认给所有节点
	if len(raftState.Peers) > 0 {
		sendCommitConfirmation(ctx, entry, raftState.Peers)
	}

	// 应用已提交的日志到状态机
	if err := raftState.ApplyCommittedLogs(); err != nil {
		log.Log.Errorf("应用日志到状态机失败: %v", err)
		return -1, err
	}

	log.Log.Infof("提交阶段成功，日志已应用到状态机，索引 %d", entry.Index)
	return raftState.GetCommitIndex(), nil
}

// sendCommitConfirmation 向所有节点发送提交确认
func sendCommitConfirmation(ctx context.Context, entry *sever.LogEntry, peers []string) {
	raftState := GetRaftState()

	// 向所有follower发送提交确认
	for _, peer := range peers {
		go func(peerAddr string) {
			// 创建一个提交确认请求
			req := &sever.AppendEntriesReq{
				Term:         raftState.GetCurrentTerm(),
				LeaderId:     raftState.GetNodeId(),
				PrevLogIndex: entry.Index - 1,
				PrevLogTerm:  entry.Term,
				Entries:      []string{},  // 空日志条目表示这是提交确认
				LeaderCommit: entry.Index, // 设置提交索引
				IsPreCommit:  false,       // 这不是预提交请求
			}

			// 获取客户端
			leaderClient := GetLeaderClient(peerAddr)
			if leaderClient == nil {
				log.Log.Errorf("无法获取节点客户端 %s", peerAddr)
				return
			}

			// 发送请求
			timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			log.Log.Infof("发送提交确认到 %s，提交日志索引 %d", peerAddr, entry.Index)

			resp, err := leaderClient.AppendEntries(timeoutCtx, req)
			if err != nil {
				log.Log.Errorf("发送提交确认失败 %s: %v", peerAddr, err)
				return
			}

			if resp.Success {
				log.Log.Infof("节点 %s 确认提交成功", peerAddr)
			} else {
				log.Log.Warnf("节点 %s 提交确认失败", peerAddr)
			}
		}(peer)
	}
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
