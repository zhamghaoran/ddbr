package client

import (
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/transport"
	"sync"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/gateway/gateway"
)

var gatewayClient gateway.Client

func GetGatewayClient() gateway.Client {
	sync.OnceFunc(createGatewayClient)
	return gatewayClient
}
func createGatewayClient() {
	gatewayClient = gateway.MustNewClient("gateway", client.WithHostPorts("0.0.0.0"), client.WithPayloadCodec(
		thrift.NewThriftCodecWithConfig(thrift.FrugalRead|thrift.FrugalWrite),
	), client.WithTransportProtocol(transport.Framed))
}
