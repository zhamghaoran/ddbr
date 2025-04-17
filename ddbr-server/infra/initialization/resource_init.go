package initialization

import (
	"context"
	"fmt"
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/gateway"
	"zhamghaoran/ddbr-server/log"
)

// InitializeResources 初始化所有资源
func InitializeResources(configPath string) error {
	im := GetInitManager()

	// 如果提供了配置路径，加载配置
	if configPath != "" {
		if err := im.LoadConfig(configPath); err != nil {
			return fmt.Errorf("failed to load config: %v", err)
		}
	}

	// 初始化Raft状态
	if err := im.InitializeRaftState(); err != nil {
		return fmt.Errorf("failed to initialize Raft state: %v", err)
	}

	// 注册服务到网关
	gatewayClient := client.GetGatewayClient()
	if gatewayClient == nil {
		return fmt.Errorf("gateway client is nil")
	}
	_, err := gatewayClient.RegisterSever(context.Background(), &gateway.RegisterSeverReq{
		ServerHost: "",
	})
	if err != nil {
		panic(err)
	}

	log.Log.Info("raft state initialized")
	return nil
}
