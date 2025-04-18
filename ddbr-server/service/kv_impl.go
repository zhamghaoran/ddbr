package service

import (
	"context"
	"fmt"
	"zhamghaoran/ddbr-server/configs"
	"zhamghaoran/ddbr-server/infra"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/common"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
)

// Set 实现设置键值对的方法
func Set(ctx context.Context, req *sever.SetReq) (*sever.SetResp, error) {
	resp := &sever.SetResp{
		Success: true,
		Message: "OK",
		Common: &common.Common{
			RespCode: 0,
			Message:  "success",
		},
	}

	// 检查是否是Leader节点
	if !configs.IsMaster() {
		// 如果不是Leader，直接返回错误
		resp.Success = false
		resp.Message = "not leader"
		resp.Common.RespCode = 1
		resp.Common.Message = "current node is not leader"
		return resp, nil
	}

	// 构造命令
	cmd := fmt.Sprintf("set:%s:%s", req.Key, req.Value)

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

	// 追加日志条目到本地日志
	logs = append(logs, logEntry)
	raftState.SetLogs(logs)

	// 复制日志到Follower节点并等待提交
	commitIndex, err := infra.ReplicateLogs(ctx, logEntry)
	if err != nil {
		resp.Success = false
		resp.Message = fmt.Sprintf("failed to replicate logs: %v", err)
		resp.Common.RespCode = 1
		resp.Common.Message = resp.Message
		return resp, nil
	}

	// 检查是否成功提交
	if commitIndex < logEntry.Index {
		// 没有足够的节点确认，无法提交
		resp.Success = false
		resp.Message = "failed to commit log: not enough acknowledgements from followers"
		resp.Common.RespCode = 1
		resp.Common.Message = resp.Message
		return resp, nil
	}

	// 日志已提交，返回成功
	resp.Message = "OK"
	return resp, nil
}

// Get 实现获取键值的方法
func Get(ctx context.Context, req *sever.GetReq) (*sever.GetResp, error) {
	resp := &sever.GetResp{
		Success: true,
		Exists:  false,
		Common: &common.Common{
			RespCode: 0,
			Message:  "success",
		},
	}

	// 检查是否是Leader节点
	if !configs.IsMaster() {
		// 如果不是Leader，直接返回错误
		resp.Success = false
		resp.Value = ""
		resp.Exists = false
		resp.Common.RespCode = 1
		resp.Common.Message = "current node is not leader"
		return resp, nil
	}

	// 从状态机中读取值
	value, exists := infra.GetStateMachine().Get(req.Key)

	resp.Exists = exists
	if exists {
		resp.Value = value
	} else {
		resp.Value = "(nil)"
	}

	return resp, nil
}

// Delete 实现删除键值对的方法
func Delete(ctx context.Context, req *sever.DeleteReq) (*sever.DeleteResp, error) {
	resp := &sever.DeleteResp{
		Success: true,
		Message: "OK",
		Common: &common.Common{
			RespCode: 0,
			Message:  "success",
		},
	}

	// 检查是否是Leader节点
	if !configs.IsMaster() {
		// 如果不是Leader，直接返回错误
		resp.Success = false
		resp.Message = "not leader"
		resp.Common.RespCode = 1
		resp.Common.Message = "current node is not leader"
		return resp, nil
	}

	// 构造命令
	cmd := fmt.Sprintf("delete:%s", req.Key)

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

	// 追加日志条目到本地日志
	logs = append(logs, logEntry)
	raftState.SetLogs(logs)

	// 复制日志到Follower节点并等待提交
	commitIndex, err := infra.ReplicateLogs(ctx, logEntry)
	if err != nil {
		resp.Success = false
		resp.Message = fmt.Sprintf("failed to replicate logs: %v", err)
		resp.Common.RespCode = 1
		resp.Common.Message = resp.Message
		return resp, nil
	}

	// 检查是否成功提交
	if commitIndex < logEntry.Index {
		// 没有足够的节点确认，无法提交
		resp.Success = false
		resp.Message = "failed to commit log: not enough acknowledgements from followers"
		resp.Common.RespCode = 1
		resp.Common.Message = resp.Message
		return resp, nil
	}

	// 日志已提交，返回成功
	resp.Message = "OK"
	return resp, nil
}
