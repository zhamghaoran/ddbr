package main

import (
	"github.com/cloudwego/kitex/client"
	"zhamghaoran/ddbr-client/kitex_gen/ddbr/rpc/gateway/gateway"
)

func InitClient(target string) gateway.Client {
	newClient := gateway.MustNewClient("gateway", client.WithHostPorts(target))
	return newClient
}
