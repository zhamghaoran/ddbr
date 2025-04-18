package client

import (
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/transport"
	"sync"
	"zhamghaoran/ddbr-server/constant"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/gateway/gateway"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever/server"
)

var gatewayClient gateway.Client

func GetGatewayClient() gateway.Client {
	onceFunc := sync.OnceFunc(createGatewayClient)
	onceFunc()
	return gatewayClient
}
func GetLeaderClient(leaderHost string) server.Client {
	serverClient := server.MustNewClient("server",
		client.WithHostPorts(leaderHost+":8080"),
		client.WithPayloadCodec(thrift.NewThriftCodecWithConfig(thrift.FrugalRead|thrift.FrugalWrite)),
		client.WithTransportProtocol(transport.Framed))
	return serverClient
}
func createGatewayClient() {
	gatewayClient = gateway.MustNewClient("gateway",
		//client.WithDestService(constant.GATEWAY_SERVER_ADDRESS),
		client.WithHostPorts(constant.GatewayServerAddress),
		client.WithPayloadCodec(thrift.NewThriftCodecWithConfig(thrift.FrugalRead|thrift.FrugalWrite)),
		client.WithTransportProtocol(transport.Framed))
}
