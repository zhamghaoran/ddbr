package repo

import (
	"sync"
	"zhamghaoran/ddbr-gateway/constants"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/gateway"
	"zhamghaoran/ddbr-gateway/log"
)

var GatewayBasicInfo gateway.GatewayBasicInfo
var gatewayRepoOnce sync.Once
var gatewayRepo *GatewayRepo

// GatewayRepo Gateway仓库结构体
type GatewayRepo struct {
	MasterHost string // 主节点地址
	leaderId   int64  // 主节点ID
	mu         sync.RWMutex
}

// GetGatewayRepo 获取Gateway仓库实例
func GetGatewayRepo() *GatewayRepo {
	gatewayRepoOnce.Do(func() {
		gatewayRepo = &GatewayRepo{
			MasterHost: info.leaderHost,
			leaderId:   -1,
		}
	})
	return gatewayRepo
}

// SetLeaderId 设置Leader节点ID
func (g *GatewayRepo) SetLeaderId(nodeId int64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.leaderId = nodeId
	// 同时更新服务器仓库
	GetSeverRepo().SetLeaderId(nodeId)
}

// GetLeaderId 获取Leader节点ID
func (g *GatewayRepo) GetLeaderId() int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.leaderId
}

// SetMasterHost 设置主节点地址
func (g *GatewayRepo) SetMasterHost(host string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.MasterHost = host
	// 同步更新老的信息
	SetLeader(host)
}

func InitGatewayInfo(info gateway.GatewayBasicInfo) error {
	if err := paramCheck(info); err != nil {
		log.Log.Errorf("InitGatewayInfo error: %v", err)
		return err
	}
	GatewayBasicInfo = info

	return nil
}

func paramCheck(info gateway.GatewayBasicInfo) error {
	if info.Password == nil {
		return constants.PasswordInfoParamError
	}
	//if len(info.SeverHostSever) == 0 {
	//	return constants.SeverHostParamError
	//}
	return nil
}
