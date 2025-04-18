package service

import (
	"context"
	"fmt"
	"strings"
	"zhamghaoran/ddbr-server/configs"
	"zhamghaoran/ddbr-server/infra"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

// ExecuteCommand 执行键值操作命令
func ExecuteCommand(ctx context.Context, cmd string) (string, error) {
	// 检查命令格式
	if cmd == "" {
		return "", fmt.Errorf("empty command")
	}

	// 只有Leader节点可以处理写命令
	if strings.HasPrefix(cmd, "set:") || strings.HasPrefix(cmd, "delete:") {
		if !configs.IsMaster() {
			// 如果不是Leader，需要转发给Leader
			return forwardCommandToLeader(ctx, cmd)
		}
	}

	// 创建日志条目
	raftState := infra.GetRaftState()
	lastIndex := int64(0)
	logs := raftState.GetLogs()
	if len(logs) > 0 {
		lastIndex = logs[len(logs)-1].Index
	}

	logEntry := &sever.LogEntry{
		Term:    raftState.GetCurrentTerm(),
		Index:   lastIndex + 1,
		Command: cmd,
	}

	// 对于读操作，直接从状态机读取
	if strings.HasPrefix(cmd, "get:") {
		parts := strings.Split(cmd, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid get command format: %s", cmd)
		}
		key := parts[1]
		value, exists := infra.GetStateMachine().Get(key)
		if !exists {
			return "(nil)", nil
		}
		return value, nil
	}

	// 追加日志条目（只有写操作需要）
	logs = append(logs, logEntry)
	raftState.SetLogs(logs)

	// 如果是Leader，应用到状态机
	result, err := infra.ApplyLogToStateMachine(*logEntry)
	if err != nil {
		return "", fmt.Errorf("failed to apply command: %v", err)
	}

	// 在实际应用中，Leader还需要复制日志到Follower
	if configs.IsMaster() {
		// TODO: 复制日志到Follower节点
		replicateLogsToFollowers(ctx, logEntry)
	}

	// 返回结果
	return result, nil
}

// forwardCommandToLeader 将命令转发给Leader节点
func forwardCommandToLeader(ctx context.Context, cmd string) (string, error) {
	// 获取Leader地址
	leaderAddr := configs.GetMasterAddr()
	if leaderAddr == "" {
		return "", fmt.Errorf("leader address not found")
	}

	// 创建一个特殊的命令格式，表示从Follower转发
	forwardCmd := fmt.Sprintf("forward:%s", cmd)

	// 在实际应用中，需要使用RPC调用Leader的ExecuteCommand接口
	log.Log.Infof("Forwarding command to leader: %s", forwardCmd)

	// 这里需要具体实现RPC调用
	// ...

	return "command forwarded to leader", nil
}

// replicateLogsToFollowers 复制日志到Follower节点
func replicateLogsToFollowers(ctx context.Context, entry *sever.LogEntry) {
	// 获取所有Follower节点
	peers := configs.GetPeers()

	// 为每个Follower创建一个异步复制任务
	for _, peer := range peers {
		// 跳过自己
		if peer == configs.GetPort() {
			continue
		}

		// 异步复制
		go func(peerAddr string) {
			// 创建AppendEntries请求
			// 在实际应用中，需要实现具体的RPC调用逻辑
			log.Log.Infof("Replicating log entry to follower %s: %+v", peerAddr, entry)

			// 这里需要具体实现RPC调用
			// ...
		}(peer)
	}
}
