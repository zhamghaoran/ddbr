package client

import (
	"github.com/cloudwego/kitex/client"
	"sync"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/gateway/gateway"
)

var gatewayClient gateway.Client

func GetGatewayClient() gateway.Client {
	sync.OnceFunc(createGatewayClient)
	return gatewayClient
}
func createGatewayClient() {
	gatewayClient = gateway.MustNewClient("gateway", client.WithHostPorts("0.0.0.0"))
}
