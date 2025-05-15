package service

import (
	"context"
	"fmt"
	"zhamghaoran/ddbr-server/configs"
	"zhamghaoran/ddbr-server/infra"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/common"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

// Set 实现设置键值对的方法
func Set(ctx context.Context, req *sever.SetReq) (*sever.SetResp, error) {
	log.Log.CtxInfof(ctx, "set receive request: %+v", req)
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
	log.Log.CtxInfof(ctx, "创建set命令: %s", cmd)

	// 创建日志条目
	raftState := infra.GetRaftState()

	// 只获取最后一个日志索引，而不是复制全部日志
	lastIndex := raftState.GetLastLogIndex()
	log.Log.CtxInfof(ctx, "获取最后日志索引: %d", lastIndex)

	// 确保日志索引从1开始
	nextIndex := lastIndex + 1
	if nextIndex < 1 {
		nextIndex = 1
	}
	log.Log.CtxInfof(ctx, "新日志索引: %d", nextIndex)

	logEntry := &sever.LogEntry{
		Term:    raftState.GetCurrentTerm(),
		Index:   nextIndex,
		Command: cmd,
	}
	log.Log.CtxInfof(ctx, "创建日志条目: {Term: %d, Index: %d, Command: %s}",
		logEntry.Term, logEntry.Index, logEntry.Command)

	// 直接追加单个日志条目，而不是操作整个日志数组
	raftState.AppendLog(logEntry)
	log.Log.CtxInfof(ctx, "追加日志条目到本地日志，当前日志长度: %d", len(raftState.GetLogs()))

	// 复制日志到Follower节点并等待提交
	log.Log.CtxInfof(ctx, "开始复制日志到Follower节点")
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
	infra.GetStateMachine().Set(req.Key, req.Value)
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

	//// 构造读取命令
	//cmd := fmt.Sprintf("get:%s", req.Key)
	//
	//// 创建日志条目
	//raftState := infra.GetRaftState()
	//
	//// 只获取最后一个日志索引，而不是复制全部日志
	//lastIndex := raftState.GetLastLogIndex()
	//
	//logEntry := &sever.LogEntry{
	//	Term:    raftState.GetCurrentTerm(),
	//	Index:   lastIndex + 1,
	//	Command: cmd,
	//	IsRead:  true, // 标记为读操作
	//}
	//
	//// 直接追加单个日志条目，而不是操作整个日志数组
	//raftState.AppendLog(logEntry)
	//
	//// 使用两阶段提交复制日志到Follower节点
	//commitIndex, err := infra.ReplicateLogs(ctx, logEntry)
	//if err != nil {
	//	resp.Success = false
	//	resp.Value = ""
	//	resp.Exists = false
	//	resp.Common.RespCode = 1
	//	resp.Common.Message = fmt.Sprintf("failed to replicate logs: %v", err)
	//	return resp, nil
	//}
	//
	//// 检查是否成功提交
	//if commitIndex < logEntry.Index {
	//	// 没有足够的节点确认，无法提交
	//	resp.Success = false
	//	resp.Value = ""
	//	resp.Exists = false
	//	resp.Common.RespCode = 1
	//	resp.Common.Message = "failed to commit log: not enough acknowledgements from followers"
	//	return resp, nil
	//}

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

	// 只获取最后一个日志索引，而不是复制全部日志
	lastIndex := raftState.GetLastLogIndex()

	// 确保日志索引从1开始
	nextIndex := lastIndex + 1
	if nextIndex < 1 {
		nextIndex = 1
	}

	logEntry := &sever.LogEntry{
		Term:    raftState.GetCurrentTerm(),
		Index:   nextIndex,
		Command: cmd,
	}

	// 直接追加单个日志条目，而不是操作整个日志数组
	raftState.AppendLog(logEntry)

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

	// 日志已提交，将操作应用到本地状态机
	infra.GetStateMachine().Delete(req.Key)

	// 添加检查代码，记录删除后的状态
	val, exist := infra.GetStateMachine().Get(req.Key)
	log.Log.CtxInfof(ctx, "after delete,val is %v,is exist :%v", val, exist)

	// 日志已提交，返回成功
	resp.Message = "OK"
	return resp, nil
}
