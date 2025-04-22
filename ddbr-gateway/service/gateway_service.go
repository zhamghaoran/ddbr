package service

import (
	"context"
	"fmt"
	"zhamghaoran/ddbr-gateway/infra"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/common"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/gateway"
	clientgateway "zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/gateway/gateway"
	"zhamghaoran/ddbr-gateway/log"
	"zhamghaoran/ddbr-gateway/repo"
	"zhamghaoran/ddbr-gateway/util"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/transport"
)

func GetGatewayInfoFromMaster(config *infra.CmdConfig) error {
	MasterGatewayClient := clientgateway.MustNewClient("gateway", client.WithHostPorts(config.MasterGatewayHost), client.WithPayloadCodec(
		thrift.NewThriftCodecWithConfig(thrift.FrugalRead|thrift.FrugalWrite),
	), client.WithTransportProtocol(transport.Framed))
	resp, err := MasterGatewayClient.RegisterGateway(
		context.Background(),
		&gateway.RegisterGatewayReq{},
	)
	if err != nil {
		log.Log.Errorf("rpc call gateway master error : %v", err)
		return err
	}
	log.Log.CtxInfof(context.Background(), resp.Info.String())
	err = repo.InitGatewayInfo(*resp.Info)
	if err != nil {
		log.Log.Errorf("repo.InitGatewayInfo error: %v", err)
		return err
	}
	SetPassword(resp.Info.Password.Password)
	return nil
}
func RegisterGatewayService(ctx context.Context, req *gateway.RegisterGatewayReq) (*gateway.RegisterGatewayResp, error) {
	log.Log.CtxInfof(ctx, "req is :%v", req)
	host, err := util.GetRemoteHost(ctx)
	if err != nil {
		return nil, err
	}
	repo.AddGatewayHost(host)
	resp := &gateway.RegisterGatewayResp{
		Info: &repo.GatewayBasicInfo,
	}
	return resp, nil
}

type GatewayService struct{}

// RegisterSever 节点服务注册
func (s *GatewayService) RegisterSever(ctx context.Context, req *gateway.RegisterSeverReq) (resp *gateway.RegisterSeverResp, err error) {
	resp = gateway.NewRegisterSeverResp()
	svrRepo := repo.GetSeverRepo()

	// 使用util.GetRemoteHost获取远程主机地址，而不是使用req.ServerHost
	host, err := util.GetRemoteHost(ctx)
	if err != nil {
		log.Log.Errorf("Failed to get remote host: %v", err)
		return nil, err
	}

	// 添加服务器到集群
	svrRepo.AddSever(host, req.NodeId)

	// 获取当前Leader信息
	gRepo := repo.GetGatewayRepo()
	resp.LeaderHost = gRepo.MasterHost
	resp.LeaderId = gRepo.GetLeaderId()

	// 返回当前集群信息
	resp.SeverHostSever = svrRepo.GetAllSevers()

	// 如果是新节点，触发日志同步通知
	if req.IsNew {
		notifyLeaderForNewNode(req.NodeId, host)
	}

	resp.Common = &common.Common{
		RespCode: 0,
		Message:  "register success",
	}

	log.Log.Infof("Server registered: %s, nodeId: %d", host, req.NodeId)
	return resp, nil
}

// notifyLeaderForNewNode 通知Leader有新节点加入
func notifyLeaderForNewNode(nodeId int64, nodeHost string) {
	// 获取Leader地址并发送通知
	// 这里需要根据实际情况实现Leader通知逻辑
	log.Log.Infof("Notifying leader about new node: %d at %s", nodeId, nodeHost)
}

func RegisterSever(ctx context.Context, req *gateway.RegisterSeverReq) (*gateway.RegisterSeverResp, error) {
	resp := gateway.NewRegisterSeverResp()
	svrRepo := repo.GetSeverRepo()

	// 使用util.GetRemoteHost获取远程主机地址，而不是使用req.ServerHost
	host, err := util.GetRemoteHost(ctx)
	if err != nil {
		log.Log.Errorf("Failed to get remote host: %v", err)
		return nil, err
	}

	// 使用ctx中获取的主机地址替代req.ServerHost
	svrRepo.AddSever(host, req.NodeId)

	// 获取当前Leader信息
	gRepo := repo.GetGatewayRepo()
	resp.LeaderHost = gRepo.MasterHost

	// 检查并更新leaderId
	currentLeaderId := gRepo.GetLeaderId()
	if currentLeaderId <= 0 && gRepo.MasterHost != "" {
		// 如果有leader但没有ID，检查当前注册的节点是否是leader
		if gRepo.MasterHost == host {
			// 如果当前注册的节点是leader节点，更新leaderId
			gRepo.SetLeaderId(req.NodeId)
			log.Log.Infof("Updated leader ID to %d", req.NodeId)
			currentLeaderId = req.NodeId
		}
	}

	resp.LeaderId = currentLeaderId

	// 返回当前集群信息
	resp.SeverHostSever = svrRepo.GetAllSevers()

	// 如果是新节点，触发日志同步通知
	if req.IsNew {
		notifyLeaderForNewNode(req.NodeId, host)
	}

	resp.Common = &common.Common{
		RespCode: 0,
		Message:  "register success",
	}

	log.Log.Infof("Server registered: %s, nodeId: %d", host, req.NodeId)
	return resp, nil
}
func SetLeader(ctx context.Context, req *gateway.SetLeaderReq) (*gateway.SetLeaderResp, error) {
	resp := &gateway.SetLeaderResp{}
	host, err := util.GetRemoteHost(ctx)
	if err != nil {
		return nil, err
	}

	// 获取当前leader信息
	gRepo := repo.GetGatewayRepo()
	currentLeaderHost := repo.GetLeaderHost()
	currentLeaderId := gRepo.GetLeaderId()

	// 检查是否已有leader
	if currentLeaderHost != "" && currentLeaderId > 0 {
		log.Log.Warnf("Leader already exists (host: %s, id: %d), rejecting new leader registration from %s",
			currentLeaderHost, currentLeaderId, host)

		// 返回当前leader信息
		resp.SetMasterHost(currentLeaderHost)

		// 返回一个自定义错误
		return resp, fmt.Errorf("leader already exists")
	}

	// 如果没有leader，则设置新的leader
	log.Log.Infof("Setting new leader: %s", host)
	repo.SetLeader(host)

	// 同时尝试获取该节点的ID并设置为leaderId
	// 这里需要从节点映射中查找
	svrRepo := repo.GetSeverRepo()
	for nodeId, nodeHost := range svrRepo.GetNodeMap() {
		if nodeHost == host {
			gRepo.SetLeaderId(nodeId)
			log.Log.Infof("Set leader ID to %d", nodeId)
			break
		}
	}

	resp.SetMasterHost(repo.GetLeaderHost())
	return resp, nil
}
