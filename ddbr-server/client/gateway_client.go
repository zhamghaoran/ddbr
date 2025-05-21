package client

import (
	"strings"
	"sync"
	"time"
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
			if leaderHost == "" {
				log.Log.Error("无法获取有效的Leader地址")
				return nil
			}
		}
	}

	// 检查主机是否包含端口，如果不包含则尝试使用配置的端口
	if !strings.Contains(leaderHost, ":") {
		port := configs.GetConfig().Port
		if port == "" {
			port = "8080" // 默认端口
		}
		leaderHost = leaderHost + ":" + port
		log.Log.Infof("补充端口信息，完整地址: %s", leaderHost)
	}

	// 检查缓存
	clientMutex.RLock()
	if c, ok := clientCache[leaderHost]; ok {
		clientMutex.RUnlock()
		return c
	}
	clientMutex.RUnlock()

	// 创建新客户端
	clientMutex.Lock()
	defer clientMutex.Unlock()

	// 再次检查缓存（避免竞态条件）
	if c, ok := clientCache[leaderHost]; ok {
		return c
	}

	log.Log.Infof("创建新的Leader客户端连接，地址: %s", leaderHost)

	// 创建新客户端连接
	serverClient, err := server.NewClient("server",
		client.WithHostPorts(leaderHost),
		client.WithPayloadCodec(thrift.NewThriftCodecWithConfig(thrift.FrugalRead|thrift.FrugalWrite)),
		client.WithTransportProtocol(transport.Framed),
		client.WithConnectTimeout(3*time.Second)) // 增加连接超时

	if err != nil {
		log.Log.Errorf("创建Leader客户端失败: %s, 错误: %v", leaderHost, err)
		return nil
	}

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
