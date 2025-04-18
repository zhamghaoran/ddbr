package infra

import (
	"context"
	"fmt"
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/configs"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/gateway"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

// SyncLogsWithLeader 与Leader同步日志
func SyncLogsWithLeader(ctx context.Context, leaderId int64) error {
	log.Log.Infof("Starting log sync with leader: %d", leaderId)

	// 如果自己就是Leader，不需要同步
	raftState := GetRaftState()
	if raftState.GetNodeId() == leaderId {
		log.Log.Info("This node is leader, no need to sync logs")
		return nil
	}

	// 获取当前日志状态
	lastLogIndex := int64(0)
	lastLogTerm := int64(0)

	logs := raftState.GetLogs()
	if len(logs) > 0 {
		lastLog := logs[len(logs)-1]
		lastLogIndex = lastLog.Index
		lastLogTerm = lastLog.Term
	}

	// 构造请求
	syncReq := &sever.LogSyncReq{
		NodeId:       raftState.GetNodeId(),
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}

	// 如果没有提供有效的leaderId，尝试从Gateway获取Leader信息
	leaderHost := ""
	if leaderId <= 0 {
		log.Log.Info("Leader ID not provided, trying to get from Gateway")

		// 从Gateway获取Leader信息
		gClient := client.GetGatewayClient()
		resp, err := gClient.RegisterSever(ctx, &gateway.RegisterSeverReq{
			ServerHost: "", // 这里可以传空，因为我们只是想获取Leader信息
			NodeId:     raftState.GetNodeId(),
			IsNew:      false,
		})

		if err != nil {
			return fmt.Errorf("failed to get leader info from gateway: %v", err)
		}
		leaderId = resp.LeaderId
		leaderHost = resp.LeaderHost
		if leaderId <= 0 {
			return fmt.Errorf("invalid leader ID from gateway: %d", leaderId)
		}
	} else {
		// 从配置或其他地方获取leader的host
		config := configs.GetConfig()
		for _, peer := range config.Peers {
			// 简单地假设peers中包含了leader的信息
			if peer != "" {
				leaderHost = peer
				break
			}
		}
		// 如果仍然没有找到leader地址，使用master地址
		if leaderHost == "" {
			leaderHost = config.MasterAddr
		}
	}

	if leaderHost == "" {
		return fmt.Errorf("failed to get leader host for leader ID: %d", leaderId)
	}

	log.Log.Infof("Connecting to leader at: %s", leaderHost)

	// 获取Leader客户端
	leaderClient := client.GetLeaderClient(leaderHost)
	if leaderClient == nil {
		return fmt.Errorf("failed to create leader client for host: %s", leaderHost)
	}

	// 调用Leader的SyncLogs接口
	log.Log.Infof("Calling SyncLogs API with request: %+v", syncReq)
	resp, err := leaderClient.SyncLogs(ctx, syncReq)
	if err != nil {
		return fmt.Errorf("failed to sync logs with leader: %v", err)
	}

	// 处理响应
	if !resp.Success {
		return fmt.Errorf("log sync failed: %s, leader: %d", resp.Message, resp.LeaderId)
	}

	// 获取返回的日志条目
	entries := resp.GetEntries()
	if len(entries) == 0 {
		log.Log.Info("No new logs to sync")
		return nil
	}

	log.Log.Infof("Received %d new log entries from leader", len(entries))

	// 从返回的日志条目中获取最新的Term
	// 在Raft协议中，日志条目包含了创建它们时的任期
	// 这些任期可以用来推断Leader当前的任期
	if len(entries) > 0 {
		latestTerm := entries[len(entries)-1].Term
		if latestTerm > raftState.GetCurrentTerm() {
			log.Log.Infof("Updating current term from %d to %d (based on received logs)",
				raftState.GetCurrentTerm(), latestTerm)
			raftState.SetCurrentTerm(latestTerm)
			// 重置投票状态
			raftState.SetVotedFor(-1)
		}
	}

	// 将新的日志条目添加到当前日志
	newLogs := make([]*sever.LogEntry, 0, len(entries))
	for _, entry := range entries {
		newLogs = append(newLogs, entry)

		// 可选：应用日志到状态机
		applyLogToStateMachine(*entry)
	}

	// 更新Raft状态
	currentLogs := raftState.GetLogs()
	// 如果有很多日志，可能需要比较复杂的合并逻辑
	// 这里做个简单实现，假设Leader发来的日志都是新的
	mergedLogs := append(currentLogs, newLogs...)
	raftState.SetLogs(mergedLogs)

	// 可选：持久化日志
	if im := GetInitManager(); im != nil {
		if err := im.PersistRaftState(); err != nil {
			log.Log.Warnf("Failed to persist raft state after log sync: %v", err)
		}
	}

	log.Log.Info("Log sync completed successfully")
	return nil
}

// applyLogToStateMachine 将日志应用到状态机
func applyLogToStateMachine(entry sever.LogEntry) {
	// 调用状态机实现
	result, err := ApplyLogToStateMachine(entry)
	if err != nil {
		log.Log.Warnf("Failed to apply log entry to state machine: %v", err)
		return
	}

	log.Log.Infof("Successfully applied log entry to state machine: index=%d, term=%d, result=%s",
		entry.Index, entry.Term, result)
}
