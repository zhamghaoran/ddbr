package main

import (
	"context"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/gateway"
	"zhamghaoran/ddbr-gateway/log"
	"zhamghaoran/ddbr-gateway/service"
)

// GatewayImpl implements the last service interface defined in the IDL.
type GatewayImpl struct{}

// Set implements the GatewayImpl interface.
func (s *GatewayImpl) Set(ctx context.Context, req *gateway.SetRequest) (resp *gateway.SetResponse, err error) {
	// TODO: Your code here...
	return
}

// Get implements the GatewayImpl interface.
func (s *GatewayImpl) Get(ctx context.Context, req *gateway.GetRequest) (resp *gateway.GetResponse, err error) {
	resp = gateway.NewGetResponse()
	resp.Val = "pong"
	return
}

// RegisterSever implements the GatewayImpl interface.
func (s *GatewayImpl) RegisterSever(ctx context.Context, req *gateway.RegisterSeverReq) (*gateway.RegisterSeverResp, error) {
	resp, err := service.RegisterSever(ctx, req)
	if err != nil {
		log.Log.CtxErrorf(ctx, "register sever error: %v", err)
		return resp, err
	}
	return resp, nil
}

// RegisterGateway implements the GatewayImpl interface.
func (s *GatewayImpl) RegisterGateway(ctx context.Context, req *gateway.RegisterGatewayReq) (*gateway.RegisterGatewayResp, error) {
	resp, err := service.RegisterGatewayService(ctx, req)
	if err != nil {
		log.Log.CtxErrorf(ctx, "RegisterGatewayService error: %v", err)
	}
	return resp, err
}

// SetLeader implements the GatewayImpl interface.
func (s *GatewayImpl) SetLeader(ctx context.Context, req *gateway.SetLeaderReq) (*gateway.SetLeaderResp, error) {
	resp, err := service.SetLeader(ctx, req)
	if err != nil {
		log.Log.CtxErrorf(ctx, "SetLeader error: %v", err)
		return nil, err
	}
	return resp, err

}
