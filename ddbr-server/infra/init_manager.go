package infra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/configs"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/gateway"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

// RaftConfig 保留此结构用于与旧代码兼容，但其功能已整合到 RaftState 和 configs.Config
type RaftConfig struct {
	NodeId          *int64   `json:"node_id"`          // 节点ID
	ClusterId       int64    `json:"cluster_id"`       // 集群ID
	Peers           []string `json:"peers"`            // 集群中所有节点的地址
	ElectionTimeout int      `json:"election_timeout"` // 选举超时时间（毫秒）
	HeartbeatPeriod int      `json:"heartbeat_period"` // 心跳周期（毫秒）
	DataDir         string   `json:"data_dir"`         // 数据目录
	SnapshotCount   int64    `json:"snapshot_count"`   // 触发快照的日志条目数
	Port            string   `json:"port"`             // 服务端口号
	GatewayHost     string   `json:"gateway_host"`     // 网关地址
	IsMaster        bool     `json:"is_master"`        // 自己是否是master
	MasterAddr      string   `json:"master_addr"`      // master 地址
}

// InitManager 初始化管理器
type InitManager struct {
	mu               sync.Mutex
	isInit           bool
	dataPath         string
	hasJoinedCluster bool
}

// 全局初始化管理器
var (
	initManager     *InitManager
	initManagerOnce sync.Once
)

// GetServerConfig 获取服务器配置 (为兼容性保留)
func GetServerConfig() RaftConfig {
	return GetRaftState().GetConfig()
}

// SetSetPeers 设置节点列表 (为兼容性保留)
func SetSetPeers(peers []string) {
	GetRaftState().SetPeers(peers)
}

// GetInitManager 获取初始化管理器实例
func GetInitManager() *InitManager {
	initManagerOnce.Do(func() {
		initManager = &InitManager{
			isInit:           false,
			dataPath:         "data", // 默认数据目录
			hasJoinedCluster: false,
		}
	})
	return initManager
}

// LoadConfig 从文件加载配置
func (im *InitManager) LoadConfig(configPath string, master bool) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	// 首先使用configs包加载配置
	if err := configs.LoadConfig(configPath); err != nil {
		return err
	}

	// 获取配置并设置master标志
	config := configs.GetConfig()
	config.IsMaster = master

	// 更新数据路径
	im.dataPath = config.DataDir

	// 更新RaftState
	raftConfig := config.ToRaftConfig().(RaftConfig)
	GetRaftState().UpdateConfig(raftConfig)

	return nil
}

// ModifyServerHost 修改服务器主机列表
func (im *InitManager) ModifyServerHost(serverHost []string) {
	GetRaftState().SetPeers(serverHost)
	// 同时更新configs中的配置
	configs.UpdatePeers(serverHost)
}

// InitializeRaftState 初始化Raft状态
func (im *InitManager) InitializeRaftState() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if im.isInit {
		return nil // 已经初始化
	}

	raftState := GetRaftState()

	// 设置数据目录
	if raftState.DataDir == "" {
		raftState.DataDir = im.dataPath
	} else {
		im.dataPath = raftState.DataDir
	}

	// 尝试从持久化存储恢复状态
	if err := im.recoverFromStorage(); err != nil {
		log.Log.Infof("No persistent state found or error loading: %v, starting fresh", err)
	}

	im.isInit = true
	return nil
}

// recoverFromStorage 从持久化存储中恢复状态
func (im *InitManager) recoverFromStorage() error {
	raftState := GetRaftState()
	statePath := filepath.Join(raftState.DataDir, "raft_state.json")

	data, err := ioutil.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	type persistentState struct {
		CurrentTerm int64      `json:"current_term"`
		VotedFor    int64      `json:"voted_for"`
		Logs        []LogEntry `json:"logs"`
	}

	var state persistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state file: %v", err)
	}

	raftState.SetCurrentTerm(state.CurrentTerm)
	raftState.SetVotedFor(state.VotedFor)
	raftState.SetLogs(state.Logs)

	return nil
}

// PersistRaftState 持久化Raft状态
func (im *InitManager) PersistRaftState() error {
	if !im.isInit {
		return fmt.Errorf("init manager not initialized")
	}

	raftState := GetRaftState()
	statePath := filepath.Join(raftState.DataDir, "raft_state.json")

	state := struct {
		CurrentTerm int64      `json:"current_term"`
		VotedFor    int64      `json:"voted_for"`
		Logs        []LogEntry `json:"logs"`
	}{
		CurrentTerm: raftState.GetCurrentTerm(),
		VotedFor:    raftState.GetVotedFor(),
		Logs:        raftState.GetLogs(),
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}

	if err := ioutil.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %v", err)
	}

	return nil
}

// HasJoinedCluster 判断是否已加入集群
func (im *InitManager) HasJoinedCluster() bool {
	im.mu.Lock()
	defer im.mu.Unlock()
	return im.hasJoinedCluster
}

// SetJoinedCluster 设置已加入集群标志
func (im *InitManager) SetJoinedCluster(joined bool) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.hasJoinedCluster = joined
}

// JoinCluster 加入集群
func (im *InitManager) JoinCluster(ctx context.Context) error {
	// 已加入集群则跳过
	if im.HasJoinedCluster() {
		return nil
	}

	// 初始化后才能加入集群
	if !im.isInit {
		return fmt.Errorf("init manager not initialized")
	}

	// 调用RegisterNodeAndGetInfo加入集群
	resp, err := RegisterNodeAndGetInfo()
	if err != nil {
		return fmt.Errorf("failed to register node: %v", err)
	}

	// 处理响应
	if resp != nil && resp.LeaderId > 0 {
		// 更新本地集群配置
		im.ModifyServerHost(resp.SeverHostSever)

		// 启动日志同步协程
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			SyncLogsWithLeader(ctx, resp.LeaderId)
		}()

		im.SetJoinedCluster(true)
		log.Log.Infof("Successfully joined the cluster, leader: %d", resp.LeaderId)
	}

	return nil
}

// InitializeResources 初始化所有资源
func InitializeResources(configPath string, master bool) error {
	im := GetInitManager()
	// 如果提供了配置路径，加载配置
	if configPath != "" {
		if err := im.LoadConfig(configPath, master); err != nil {
			return fmt.Errorf("failed to load config: %v", err)
		}
	}
	// 初始化Raft状态
	if err := im.InitializeRaftState(); err != nil {
		return fmt.Errorf("failed to initialize Raft state: %v", err)
	}

	// 向网关注册并加入集群
	if err := im.JoinCluster(context.Background()); err != nil {
		log.Log.Warnf("Failed to join cluster: %v", err)
		// 注册失败不影响本地节点启动
	}

	log.Log.Info("raft state initialized")
	return nil
}

// RegisterNodeAndGetInfo 向Gateway注册并获取集群信息
func RegisterNodeAndGetInfo() (*gateway.RegisterSeverResp, error) {
	// 注册服务到网关
	raftState := GetRaftState()
	config := configs.GetConfig()
	gatewayClient := client.GetGatewayClient()

	if gatewayClient == nil {
		return nil, fmt.Errorf("gateway client is nil")
	}

	ctx := context.Background()

	// 向Gateway注册
	resp, err := gatewayClient.RegisterSever(ctx, &gateway.RegisterSeverReq{
		ServerHost: config.Port,
		NodeId:     raftState.GetNodeId(),
		IsNew:      !config.IsMaster, // 如果不是master，则标记为新节点
	})
	if err != nil {
		return nil, err
	}

	// 如果是master，向网关注册成为leader
	if config.IsMaster {
		_, err := gatewayClient.SetLeader(ctx, &gateway.SetLeaderReq{})
		if err != nil {
			log.Log.Errorf("Failed to set leader: %v", err)
			return resp, err
		}
	} else if resp != nil && resp.LeaderHost != "" {
		// 如果不是master且获取到了leader地址，向master注册
		masterClient := client.GetLeaderClient(resp.LeaderHost)

		// 更新主节点地址
		configs.SetMasterAddr(resp.LeaderHost)

		if masterClient != nil {
			_, err := masterClient.JoinCluster(ctx, &sever.JoinClusterReq{})
			if err != nil {
				log.Log.Errorf("Failed to join cluster: %v", err)
				return resp, err
			}
		}
	}
	return resp, nil
}
