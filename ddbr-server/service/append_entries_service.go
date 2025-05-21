package service

import (
	"context"
	"zhamghaoran/ddbr-server/infra"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

// HandleAppendEntries 处理追加日志/心跳请求
func HandleAppendEntries(ctx context.Context, req *sever.AppendEntriesReq) (resp *sever.AppendEntriesResp, err error) {
	log.Log.CtxInfof(ctx, "接收到追加日志/心跳请求: Term=%d, LeaderId=%d, Entries=%d",
		req.Term, req.LeaderId, len(req.Entries))

	// 使用状态机处理请求
	stateMachine := infra.GetRaftStateMachine()
	resp, err = stateMachine.AppendEntries(ctx, req)

	// 记录处理结果
	if err == nil {
		log.Log.CtxInfof(ctx, "追加日志/心跳请求处理成功: Term=%d, Success=%v",
			resp.Term, resp.Success)
	} else {
		log.Log.CtxErrorf(ctx, "追加日志/心跳请求处理失败: %v", err)
		// 生成默认的错误响应
		resp = &sever.AppendEntriesResp{
			Term:    stateMachine.GetCurrentTerm(),
			Success: false,
		}
	}

	return resp, err
}
