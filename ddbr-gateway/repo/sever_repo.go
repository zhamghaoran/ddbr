package repo

import (
	"sync"
)

type SeverInfo struct {
	severList  []string
	leaderHost string
	nodeMap    map[int64]string // 节点ID到地址的映射
	leaderId   int64            // Leader节点ID
	mu         sync.RWMutex     // 用于并发安全
}

type GatewayInfo struct {
	gatewayHost map[string]struct{}
}

var gatewayInfo GatewayInfo
var info SeverInfo
var severRepoOnce sync.Once
var severRepo *SeverRepo

// SeverRepo 服务器仓库
type SeverRepo struct {
	info *SeverInfo
}

// GetSeverRepo 获取服务器仓库实例
func GetSeverRepo() *SeverRepo {
	severRepoOnce.Do(func() {
		severRepo = &SeverRepo{
			info: &info,
		}
	})
	return severRepo
}

func init() {
	info = SeverInfo{
		severList:  make([]string, 0),
		leaderHost: "",
		nodeMap:    make(map[int64]string),
		leaderId:   -1,
	}
	gatewayInfo = GatewayInfo{
		gatewayHost: make(map[string]struct{}),
	}
}

// AddSever 添加服务器节点，带节点ID
func (s *SeverRepo) AddSever(host string, nodeId int64) {
	s.info.mu.Lock()
	defer s.info.mu.Unlock()

	// 检查是否已存在
	for _, h := range s.info.severList {
		if h == host {
			// 已存在则更新nodeId映射
			s.info.nodeMap[nodeId] = host
			return
		}
	}

	// 不存在则添加
	s.info.severList = append(s.info.severList, host)
	s.info.nodeMap[nodeId] = host
}

// GetAllSevers 获取所有服务器地址
func (s *SeverRepo) GetAllSevers() []string {
	s.info.mu.RLock()
	defer s.info.mu.RUnlock()

	result := make([]string, len(s.info.severList))
	copy(result, s.info.severList)
	return result
}

// GetSeverById 根据节点ID获取服务器地址
func (s *SeverRepo) GetSeverById(nodeId int64) string {
	s.info.mu.RLock()
	defer s.info.mu.RUnlock()

	return s.info.nodeMap[nodeId]
}

// SetLeaderId 设置Leader节点ID
func (s *SeverRepo) SetLeaderId(nodeId int64) {
	s.info.mu.Lock()
	defer s.info.mu.Unlock()

	s.info.leaderId = nodeId
}

// GetLeaderId 获取Leader节点ID
func (s *SeverRepo) GetLeaderId() int64 {
	s.info.mu.RLock()
	defer s.info.mu.RUnlock()

	return s.info.leaderId
}

// 保留原有函数以保持兼容性
func AddServer(host string) {
	GetSeverRepo().AddSever(host, -1) // 使用-1表示未知节点ID
}

func SetLeader(host string) {
	info.leaderHost = host
}

func GetSeverList() []string {
	return GetSeverRepo().GetAllSevers()
}

func GetLeaderHost() string {
	return info.leaderHost
}

func AddGatewayHost(host string) {
	gatewayInfo.gatewayHost[host] = struct{}{}
}

func GetGatewayList() []string {
	gatewayList := make([]string, 0)
	for k := range gatewayInfo.gatewayHost {
		gatewayList = append(gatewayList, k)
	}
	return gatewayList
}
