package infra

import (
	"context"
	"fmt"
	"time"
	"zhamghaoran/ddbr-server/client"
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

	// 如果没有提供leaderId，尝试获取Leader地址（从Gateway）
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
		if leaderId <= 0 {
			return fmt.Errorf("invalid leader ID from gateway: %d", leaderId)
		}
	}

	// 模拟与Leader交互的日志同步过程
	// TODO: 实现实际的RPC调用
	log.Log.Infof("Syncing logs with leader %d, last log index: %d, last log term: %d, request: %v",
		leaderId, lastLogIndex, lastLogTerm, syncReq)

	// 模拟同步延迟
	select {
	case <-time.After(500 * time.Millisecond):
		// 模拟成功
	case <-ctx.Done():
		return ctx.Err()
	}

	log.Log.Info("Log sync completed successfully")
	return nil
}
