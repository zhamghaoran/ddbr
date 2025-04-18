package client

import (
	"sync"
	"zhamghaoran/ddbr-server/configs"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/gateway/gateway"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever/server"
	"zhamghaoran/ddbr-server/log"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/transport"
)

var (
	gatewayClient gateway.Client
	gatewayOnce   sync.Once
	clientCache   = make(map[string]server.Client)
	clientMutex   sync.RWMutex
)

// GetGatewayClient 获取网关客户端
func GetGatewayClient() gateway.Client {
	gatewayOnce.Do(func() {
		createGatewayClient()
	})
	return gatewayClient
}

// GetLeaderClient 获取Leader客户端，支持缓存
func GetLeaderClient(leaderHost string) server.Client {
	if leaderHost == "" {
		leaderHost = configs.GetMasterAddr()
		if leaderHost == "" {
			log.Log.Warn("Leader host is empty, using gateway host")
			leaderHost = configs.GetGatewayHost()
		}
	}

	// 检查缓存
	clientMutex.RLock()
	if client, ok := clientCache[leaderHost]; ok {
		clientMutex.RUnlock()
		return client
	}
	clientMutex.RUnlock()

	// 创建新客户端
	clientMutex.Lock()
	defer clientMutex.Unlock()

	// 再次检查缓存（避免竞态条件）
	if client, ok := clientCache[leaderHost]; ok {
		return client
	}

	// 使用端口配置，如果没有指定则使用默认端口
	port := ":8080"
	serverClient := server.MustNewClient("server",
		client.WithHostPorts(leaderHost+port),
		client.WithPayloadCodec(thrift.NewThriftCodecWithConfig(thrift.FrugalRead|thrift.FrugalWrite)),
		client.WithTransportProtocol(transport.Framed))

	// 添加到缓存
	clientCache[leaderHost] = serverClient
	return serverClient
}

// 创建网关客户端
func createGatewayClient() {
	gatewayHost := configs.GetGatewayHost()
	if gatewayHost == "" {
		gatewayHost = "localhost:8888" // 默认网关地址
		log.Log.Warn("Gateway host not configured, using default: ", gatewayHost)
	}

	gatewayClient = gateway.MustNewClient("gateway",
		client.WithHostPorts(gatewayHost),
		client.WithPayloadCodec(thrift.NewThriftCodecWithConfig(thrift.FrugalRead|thrift.FrugalWrite)),
		client.WithTransportProtocol(transport.Framed))
}

// ClearClientCache 清除客户端缓存
func ClearClientCache() {
	clientMutex.Lock()
	defer clientMutex.Unlock()
	clientCache = make(map[string]server.Client)
}
