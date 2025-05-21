package client

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/sever/server"
	"zhamghaoran/ddbr-gateway/log"
	"zhamghaoran/ddbr-gateway/repo"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/transport"
)

// serverClientPool 用于缓存和管理Server客户端连接
var (
	serverClientPool     = make(map[string]server.Client)
	serverClientPoolLock sync.RWMutex
	defaultTimeout       = 3 * time.Second
	defaultConnTimeout   = 1 * time.Second
)

// GetServerClient 获取或创建Server客户端
func GetServerClient() (server.Client, error) {
	svrRepo := repo.GetSeverRepo()
	leaderHost := svrRepo.GetLeaderHost()

	if leaderHost == "" {
		log.Log.Errorf("获取Leader地址失败，当前没有可用的Leader节点")
		return nil, fmt.Errorf("无可用的Leader节点")
	}

	// 获取地址中是否包含端口
	var hostWithPort string
	if strings.Contains(leaderHost, ":") {
		hostWithPort = leaderHost
	} else {
		hostWithPort = leaderHost + ":8080" // 默认端口
	}

	// 先从连接池中查找客户端
	serverClientPoolLock.RLock()
	cli, exists := serverClientPool[hostWithPort]
	serverClientPoolLock.RUnlock()

	if exists {
		log.Log.Debugf("复用Server客户端连接：%s", hostWithPort)
		return cli, nil
	}

	// 连接池中不存在，创建新客户端
	log.Log.Infof("创建新的Server客户端连接，LeaderHost: %s", hostWithPort)

	// 创建客户端配置
	clientOpts := []client.Option{
		client.WithHostPorts(hostWithPort),
		client.WithPayloadCodec(thrift.NewThriftCodecWithConfig(thrift.FrugalRead | thrift.FrugalWrite)),
		client.WithTransportProtocol(transport.Framed),
		client.WithRPCTimeout(defaultTimeout),
		client.WithConnectTimeout(defaultConnTimeout),
	}

	// 尝试创建客户端
	cli, err := server.NewClient("server", clientOpts...)
	if err != nil {
		log.Log.Errorf("创建Server客户端失败: %v, LeaderHost: %s", err, hostWithPort)
		return nil, err
	}

	// 将新客户端添加到连接池
	serverClientPoolLock.Lock()
	serverClientPool[hostWithPort] = cli
	serverClientPoolLock.Unlock()

	return cli, nil
}

// MustGetServerClient 获取服务器客户端，失败时panic
func MustGetServerClient() server.Client {
	cli, err := GetServerClient()
	if err != nil {
		panic(fmt.Sprintf("获取Server客户端失败: %v", err))
	}
	return cli
}

// UpdateLeaderClient 更新Leader客户端 - 当Leader变更时调用
func UpdateLeaderClient(newLeaderHost string) error {
	if newLeaderHost == "" {
		return fmt.Errorf("新Leader地址为空")
	}

	// 获取地址中是否包含端口
	var hostWithPort string
	if strings.Contains(newLeaderHost, ":") {
		hostWithPort = newLeaderHost
	} else {
		hostWithPort = newLeaderHost + ":8080" // 默认端口
	}

	// 检查是否已有连接
	serverClientPoolLock.RLock()
	_, exists := serverClientPool[hostWithPort]
	serverClientPoolLock.RUnlock()

	if exists {
		log.Log.Infof("已有Leader客户端连接：%s", hostWithPort)
		return nil
	}

	// 创建新的客户端
	log.Log.Infof("为新Leader创建客户端连接：%s", hostWithPort)
	clientOpts := []client.Option{
		client.WithHostPorts(hostWithPort),
		client.WithPayloadCodec(thrift.NewThriftCodecWithConfig(thrift.FrugalRead | thrift.FrugalWrite)),
		client.WithTransportProtocol(transport.Framed),
		client.WithRPCTimeout(defaultTimeout),
		client.WithConnectTimeout(defaultConnTimeout),
	}

	// 尝试创建客户端
	cli, err := server.NewClient("server", clientOpts...)
	if err != nil {
		log.Log.Errorf("创建新Leader客户端失败: %v, LeaderHost: %s", err, hostWithPort)
		return err
	}

	// 将新客户端添加到连接池
	serverClientPoolLock.Lock()
	serverClientPool[hostWithPort] = cli
	serverClientPoolLock.Unlock()

	return nil
}

// CleanClientPool 清理客户端连接池
func CleanClientPool() {
	serverClientPoolLock.Lock()
	defer serverClientPoolLock.Unlock()

	// 清空连接池
	serverClientPool = make(map[string]server.Client)
	log.Log.Infof("已清理Server客户端连接池")
}
