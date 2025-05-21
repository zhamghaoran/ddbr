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
	log.Log.CtxInfof(ctx, "接收到投票请求: %+v", req)

	// 使用状态机处理投票请求
	stateMachine := infra.GetRaftStateMachine()
	resp, err = stateMachine.RequestVote(ctx, req)

	// 记录投票结果
	if resp != nil {
		log.Log.CtxInfof(ctx, "投票请求处理结果: %+v", resp)
	} else {
		log.Log.CtxErrorf(ctx, "投票请求处理失败: %v", err)
		// 生成默认的错误响应
		resp = &sever.RequestVoteResp{
			Common: &common.Common{
				RespCode: 1,
				Message:  "internal error: " + err.Error(),
			},
			VoteGranted: false,
		}
	}

	return resp, err
}

// BeginVote 启动选举过程
func BeginVote(ctx context.Context) {
	log.Log.CtxInfof(ctx, "手动触发选举过程")

	// 获取状态机并触发选举
	stateMachine := infra.GetRaftStateMachine()
	stateMachine.BeginVote()

	log.Log.CtxInfof(ctx, "选举过程已触发")
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
