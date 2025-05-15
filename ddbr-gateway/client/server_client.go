package client

import (
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/transport"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/sever/server"
	"zhamghaoran/ddbr-gateway/log"
	"zhamghaoran/ddbr-gateway/repo"
)

func GetServerClient() server.Client {
	svrRepo := repo.GetSeverRepo()
	leaderHost := svrRepo.GetLeaderHost()
	log.Log.Infof("GetServerClient() leaderHost: %s", leaderHost)
	return server.MustNewClient("server", client.WithHostPorts(leaderHost+":8080"), client.WithPayloadCodec(
		thrift.NewThriftCodecWithConfig(thrift.FrugalRead|thrift.FrugalWrite),
	), client.WithTransportProtocol(transport.Framed))
}
