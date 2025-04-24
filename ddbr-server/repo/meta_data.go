package repo

import (
	"sync"
)

type MetaData struct {
	GatewayHost string
	ServerHost  []string
}

var (
	data     MetaData
	dataLock sync.RWMutex
)

// SetMetaData 设置元数据
func SetMetaData(metaData MetaData) {
	dataLock.Lock()
	defer dataLock.Unlock()
	data = metaData
}

// GetMetaData 获取元数据
func GetMetaData() MetaData {
	dataLock.RLock()
	defer dataLock.RUnlock()
	return data
}

// AddServerHost 添加服务器主机
func AddServerHost(host string) {
	dataLock.Lock()
	defer dataLock.Unlock()

	// 检查是否已存在
	for _, h := range data.ServerHost {
		if h == host {
			return // 已存在，不重复添加
		}
	}

	// 添加新主机
	data.ServerHost = append(data.ServerHost, host)
}

// SetGatewayHost 设置网关主机
func SetGatewayHost(host string) {
	dataLock.Lock()
	defer dataLock.Unlock()
	data.GatewayHost = host
}

// GetServerHosts 获取所有服务器主机
func GetServerHosts() []string {
	dataLock.RLock()
	defer dataLock.RUnlock()

	// 返回副本以避免外部修改
	result := make([]string, len(data.ServerHost))
	copy(result, data.ServerHost)
	return result
}
