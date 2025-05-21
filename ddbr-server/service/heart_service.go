package service

import (
	"context"
	"time"
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/infra"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/common"
	sever "zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
	"zhamghaoran/ddbr-server/repo"
	"zhamghaoran/ddbr-server/utils"
)

func HeartBeat(ctx context.Context, req *sever.HeartbeatReq) (*sever.Heartbeatresp, error) {
	// 获取远程主机地址
	remoteHost, err := utils.GetRemoteHost(ctx)
	if err != nil {
		log.Log.CtxErrorf(ctx, "获取远程主机地址失败: %v", err)
	} else {
		log.Log.CtxInfof(ctx, "接收到心跳检测请求，远程主机: %s", remoteHost)
	}

	// 获取最新的集群节点列表
	peers := repo.GetServerHosts()

	// 确保当前节点也在列表中
	currentHost := utils.GetLocalHost()
	if currentHost != "" && !contains(peers, currentHost) {
		peers = append(peers, currentHost)
	}

	// 确保发送心跳检测的节点在列表中
	if remoteHost != "" && !contains(peers, remoteHost) {
		peers = append(peers, remoteHost)
		// 更新存储的节点列表
		repo.AddServerHost(remoteHost)

		// 更新Raft状态中的节点列表
		raftState := infra.GetRaftState()
		if raftState != nil {
			raftState.SetPeers(peers)
			log.Log.CtxInfof(ctx, "更新Raft节点列表: %v", peers)
		}
	}

	log.Log.CtxInfof(ctx, "返回心跳响应，当前集群节点列表: %v", peers)

	// 创建响应
	resp := &sever.Heartbeatresp{
		Peers: peers,
		Common: &common.Common{
			RespCode: 0,
			Message:  "success",
		},
	}

	return resp, nil
}

// contains 检查列表中是否包含指定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func JoinCluster(ctx context.Context, req *sever.JoinClusterReq) (*sever.JoinClusterResp, error) {
	host, err := utils.GetRemoteHost(ctx)
	if err != nil {
		log.Log.Errorf("获取远程主机地址失败: %v", err)
		return nil, err
	}
	log.Log.Infof("接收到节点加入请求，远程主机地址: %s", host)

	// 将新节点添加到集群成员列表
	repo.AddServerHost(host)

	// 获取当前集群成员列表
	currentHosts := repo.GetServerHosts()
	log.Log.Infof("更新后的集群成员: %v", currentHosts)

	// 更新Raft状态中的节点列表
	raftState := infra.GetRaftState()
	raftState.SetPeers(currentHosts)

	return sever.NewJoinClusterResp(), nil
}

// syncLogsToNewNode 将日志同步到新节点
func syncLogsToNewNode(nodeHost string) {
	// 获取当前日志
	raftState := infra.GetRaftState()
	logs := raftState.GetLogs()

	if len(logs) == 0 {
		log.Log.Info("当前没有日志需要同步")
		return
	}

	// 创建新节点的客户端
	nodeClient := client.GetLeaderClient(nodeHost)
	if nodeClient == nil {
		log.Log.Errorf("无法创建到节点 %s 的客户端连接", nodeHost)
		return
	}

	// 构建日志同步请求
	syncReq := &sever.LogSyncReq{
		NodeId:       raftState.GetNodeId(),
		LastLogIndex: int64(len(logs) - 1),
		LastLogTerm:  logs[len(logs)-1].Term,
	}

	// 发送同步请求
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	resp, err := nodeClient.SyncLogs(ctx, syncReq)
	if err != nil {
		log.Log.Errorf("向节点 %s 同步日志失败: %v", nodeHost, err)
		return
	}

	if !resp.Success {
		log.Log.Warnf("节点 %s 同步日志响应失败: %s", nodeHost, resp.Message)
		return
	}

	log.Log.Infof("成功向节点 %s 同步日志，共 %d 条日志", nodeHost, len(resp.Entries))
}
