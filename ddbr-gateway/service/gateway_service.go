package service

import (
	"context"
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

	// 添加服务器到集群
	svrRepo.AddSever(req.ServerHost, req.NodeId)

	// 获取当前Leader信息
	gRepo := repo.GetGatewayRepo()
	resp.LeaderHost = gRepo.MasterHost
	resp.LeaderId = gRepo.GetLeaderId()

	// 返回当前集群信息
	resp.SeverHostSever = svrRepo.GetAllSevers()

	// 如果是新节点，触发日志同步通知
	if req.IsNew {
		notifyLeaderForNewNode(req.NodeId, req.ServerHost)
	}

	resp.Common = &common.Common{
		RespCode: 0,
		Message:  "register success",
	}

	log.Log.Infof("Server registered: %s, nodeId: %d", req.ServerHost, req.NodeId)
	return resp, nil
}

// notifyLeaderForNewNode 通知Leader有新节点加入
func notifyLeaderForNewNode(nodeId int64, nodeHost string) {
	// 获取Leader地址并发送通知
	// 这里需要根据实际情况实现Leader通知逻辑
	log.Log.Infof("Notifying leader about new node: %d at %s", nodeId, nodeHost)
}

func RegisterSever(ctx context.Context, req *gateway.RegisterSeverReq) (*gateway.RegisterSeverResp, error) {
	resp = gateway.NewRegisterSeverResp()
	svrRepo := repo.GetSeverRepo()

	// 添加服务器到集群
	svrRepo.AddSever(req.ServerHost, req.NodeId)

	// 获取当前Leader信息
	gRepo := repo.GetGatewayRepo()
	resp.LeaderHost = gRepo.MasterHost
	resp.LeaderId = gRepo.GetLeaderId() // 需要实现这个方法

	// 返回当前集群信息
	resp.SeverHostSever = svrRepo.GetAllSevers()

	// 如果是新节点，触发日志同步通知
	if req.IsNew {
		notifyLeaderForNewNode(req.NodeId, req.ServerHost)
	}

	resp.Common = &common.Common{
		RespCode: 0,
		Message:  "register success",
	}

	log.Log.Infof("Server registered: %s, nodeId: %d", req.ServerHost, req.NodeId)
	return resp, nil
}
func SetLeader(ctx context.Context, req *gateway.SetLeaderReq) (*gateway.SetLeaderResp, error) {
	resp := &gateway.SetLeaderResp{}
	host, err := util.GetRemoteHost(ctx)
	if err != nil {
		return nil, err
	}
	repo.SetLeader(host)
	resp.SetMasterHost(repo.GetLeaderHost())
	return resp, nil
}
