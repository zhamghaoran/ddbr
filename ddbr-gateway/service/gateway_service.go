package service

import (
	"context"
	"github.com/cloudwego/kitex/client"
	"zhamghaoran/ddbr-gateway/infra"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/gateway"
	clientgateway "zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/gateway/gateway"
	"zhamghaoran/ddbr-gateway/log"
	"zhamghaoran/ddbr-gateway/repo"
)

func GetGatewayInfoFromMaster(config *infra.CmdConfig) error {
	MasterGatewayClient := clientgateway.MustNewClient("gateway", client.WithHostPorts(config.MasterGatewayHost))
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
	resp := &gateway.RegisterGatewayResp{
		Info: &repo.GatewayBasicInfo,
	}
	return resp, nil
}
func RegisterSever(ctx context.Context, req *gateway.RegisterSeverReq) (*gateway.RegisterSeverResp, error) {
	repo.AddServer(req.ServerHost)
	resp := &gateway.RegisterSeverResp{}
	return resp, nil
}
func SetLeader(ctx context.Context, req *gateway.SetLeaderReq) (*gateway.SetLeaderResp, error) {
	resp := &gateway.SetLeaderResp{}
	repo.SetLeader(req.LeaderHost)
	return resp, nil
}
