package service

import (
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/common"
)

// KVOperationReq 键值操作请求
type KVOperationReq struct {
	Command string         // 命令: "set:key:value", "get:key", "delete:key"
	Common  *common.Common // 通用字段
}

// KVOperationResp 键值操作响应
type KVOperationResp struct {
	Success bool           // 操作是否成功
	Result  string         // 操作结果
	Common  *common.Common // 通用字段
}

// KVOperation 处理键值操作请求
