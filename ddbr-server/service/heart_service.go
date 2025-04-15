package service

import (
	"context"
	sever "zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/repo"
)

func HeartBeat(ctx context.Context, req *sever.HeartbeatReq) (*sever.Heartbeatresp, error) {
	return &sever.Heartbeatresp{
		Peers: repo.GetMetaData().ServerHost,
	}, nil
}
