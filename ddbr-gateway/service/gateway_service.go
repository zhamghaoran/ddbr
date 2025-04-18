package service

import (
	"context"
	"zhamghaoran/ddbr-gateway/infra"
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
func RegisterSever(ctx context.Context, req *gateway.RegisterSeverReq) (*gateway.RegisterSeverResp, error) {
	host, err := util.GetRemoteHost(ctx)
	if err != nil {
		return nil, err
	}
	repo.AddServer(host)
	resp := &gateway.RegisterSeverResp{}
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
