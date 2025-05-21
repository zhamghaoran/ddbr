package utils

import (
	"context"
	"fmt"
	"net"
	"zhamghaoran/ddbr-server/configs"
	"zhamghaoran/ddbr-server/log"

	"github.com/cloudwego/kitex/pkg/utils/kitexutil"
)

func GetRemoteHost(ctx context.Context) (string, error) {
	cluster, ok := kitexutil.GetCallerAddr(ctx)
	if !ok {
		return "", fmt.Errorf("caller addr error")
	}
	host, _, err := net.SplitHostPort(cluster.String())
	if err != nil {
		return "", err
	}
	log.Log.CtxInfof(ctx, "remote server host is %s", host)
	return host, nil
}

// GetLocalHost 获取本地服务器地址
func GetLocalHost() string {
	config := configs.GetConfig()

	// 尝试从配置中获取本地地址
	if config.MasterAddr != "" && config.IsMaster {
		return config.MasterAddr
	}

	// 直接返回IP+端口
	host, err := getOutboundIP()
	if err != nil {
		log.Log.Warnf("获取本地IP地址失败: %v", err)
		return "" // 无法获取本地地址
	}

	// 添加端口
	if config.Port != "" {
		return host + ":" + config.Port
	}

	return host
}

// getOutboundIP 获取本地对外IP地址
func getOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
