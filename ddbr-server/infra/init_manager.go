package infra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/apache/thrift/lib/go/thrift"
	"github.com/google/uuid"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/gateway"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

// InitManager 初始化管理器
type InitManager struct {
	mu       sync.Mutex
	config   *RaftConfig
	isInit   bool
	dataPath string
}

// 全局初始化管理器
var (
	initManager     *InitManager
	initManagerOnce sync.Once
)

// GetServerConfig 获取服务器配置
func GetServerConfig() RaftConfig {
	manager := GetInitManager()
	config := manager.config
	return *config
}
func SetSetPeers(peers []string) {
	GetInitManager().config.Peers = peers
}

// GetInitManager 获取初始化管理器实例
func GetInitManager() *InitManager {
	initManagerOnce.Do(func() {
		initManager = &InitManager{
			isInit:   false,
			dataPath: "data", // 默认数据目录
		}
	})
	return initManager
}

// LoadConfig 从文件加载配置
func (im *InitManager) LoadConfig(configPath string, master bool) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	var config RaftConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}
	im.config = &config
	im.config.IsMaster = master
	im.dataPath = config.DataDir
	if im.config.NodeId == nil {
		im.config.NodeId = thrift.Int64Ptr(int64(uuid.New().ID()))
	}
	return nil
}

// ModifyServerHost 修改服务器主机列表
func (im *InitManager) ModifyServerHost(serverHost []string) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.config.Peers = serverHost
}

// InitializeRaftState 初始化Raft状态
func (im *InitManager) InitializeRaftState() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if im.isInit {
		return nil // 已经初始化
	}

	if im.config == nil {
		// 如果没有加载配置，使用默认配置
		im.config = &RaftConfig{
			NodeId:          thrift.Int64Ptr(GetRaftState().GetNodeId()),
			ClusterId:       1,
			ElectionTimeout: 1000,
			HeartbeatPeriod: 100,
			DataDir:         im.dataPath,
			SnapshotCount:   10000,
		}
	} else {
		// 使用配置文件中的节点ID
		GetRaftState().SetNodeId(*im.config.NodeId)
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
	statePath := filepath.Join(im.config.DataDir, "raft_state.json")

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

	raftState := GetRaftState()
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

	statePath := filepath.Join(im.config.DataDir, "raft_state.json")

	raftState := GetRaftState()
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

// GetRaftConfig 获取Raft配置
func (im *InitManager) GetRaftConfig() *RaftConfig {
	im.mu.Lock()
	defer im.mu.Unlock()

	if im.config == nil {
		return nil
	}

	// 返回配置的副本，避免外部修改
	configCopy := *im.config
	return &configCopy
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
	// 向gateway 节点和 master 节点注册
	if err := RegisterNode(); err != nil {
		return fmt.Errorf("failed to register node: %v", err)
	}

	// 如果不是master节点，尝试获取集群信息并同步日志
	if !master {
		// 获取当前配置
		config := GetServerConfig()

		// 从Gateway获取集群信息
		if err := updateClusterInfo(config); err != nil {
			log.Log.Warnf("Join cluster failed: %v, will retry later", err)
			// 失败不阻止启动，后续会定期重试
		}
	}

	log.Log.Info("raft state initialized")
	return nil
}

// updateClusterInfo 更新集群信息并启动日志同步
func updateClusterInfo(config RaftConfig) error {
	gatewayClient := client.GetGatewayClient()
	// 注册服务并获取集群信息
	resp, err := gatewayClient.RegisterSever(context.Background(), &gateway.RegisterSeverReq{
		ServerHost: config.Port, // 使用配置的端口
		NodeId:     *config.NodeId,
		IsNew:      true, // 标记为新节点
	})
	if err != nil {
		return err
	}
	// 获取集群信息
	if resp.Common != nil && resp.Common.RespCode == 0 {
		// 更新本地集群配置
		im := GetInitManager()
		im.ModifyServerHost(resp.SeverHostSever)

		// 启动日志同步协程
		go syncLogsWithLeaderAsync(resp.LeaderId)

		log.Log.Infof("Successfully joined the cluster, leader: %d, peers: %v",
			resp.LeaderId, resp.SeverHostSever)
	}

	return nil
}

// syncLogsWithLeaderAsync 异步与Leader同步日志
func syncLogsWithLeaderAsync(leaderId int64) {
	// 如果没有提供有效的Leader ID，返回
	if leaderId <= 0 {
		log.Log.Warn("Invalid leader ID, skip log sync")
		return
	}

	// 尝试与Leader同步日志
	err := SyncLogsWithLeader(context.Background(), leaderId)
	if err != nil {
		log.Log.Warnf("Failed to sync logs with leader: %v", err)
	}
}

func RegisterNode() error {
	// 注册服务到网关
	config := GetServerConfig()
	gatewayClient := client.GetGatewayClient()
	if gatewayClient == nil {
		return fmt.Errorf("gateway client is nil")
	}
	ctx := context.Background()
	resp, err := gatewayClient.RegisterSever(ctx, &gateway.RegisterSeverReq{})
	// 自己是master，向网关注册
	if config.IsMaster {
		_, err := gatewayClient.SetLeader(ctx, &gateway.SetLeaderReq{})
		if err != nil {
			panic(err)
		}
	} else {
		masterClient := client.GetLeaderClient(resp.GetLeaderHost())
		_, err := masterClient.JoinCluster(ctx, &sever.JoinClusterReq{})
		if err != nil {
			return err
		}
	}
	if err != nil {
		panic(err)
	}
	return err
}
