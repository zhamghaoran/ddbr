package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"zhamghaoran/ddbr-server/log"
)

// RaftConfig 表示Raft节点配置
type RaftConfig struct {
	NodeId          int64    `json:"node_id"`          // 节点ID
	ClusterId       int64    `json:"cluster_id"`       // 集群ID
	Peers           []string `json:"peers"`            // 集群中所有节点的地址
	ElectionTimeout int      `json:"election_timeout"` // 选举超时时间（毫秒）
	HeartbeatPeriod int      `json:"heartbeat_period"` // 心跳周期（毫秒）
	DataDir         string   `json:"data_dir"`         // 数据目录
	SnapshotCount   int64    `json:"snapshot_count"`   // 触发快照的日志条目数
}

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
func (im *InitManager) LoadConfig(configPath string) error {
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
	im.dataPath = config.DataDir
	return nil
}
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
			NodeId:          raftState.nodeId,
			ClusterId:       1,
			ElectionTimeout: 1000,
			HeartbeatPeriod: 100,
			DataDir:         im.dataPath,
			SnapshotCount:   10000,
		}
	} else {
		// 使用配置文件中的节点ID
		raftState.nodeId = im.config.NodeId
	}

	//// 确保数据目录存在
	//if err := os.MkdirAll(im.config.DataDir, 0755); err != nil {
	//	return fmt.Errorf("failed to create data directory: %v", err)
	//}

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

	raftState.mu.Lock()
	raftState.currentTerm = state.CurrentTerm
	raftState.votedFor = state.VotedFor
	raftState.logs = state.Logs
	raftState.mu.Unlock()

	return nil
}

// PersistRaftState 持久化Raft状态
func (im *InitManager) PersistRaftState() error {
	if !im.isInit {
		return fmt.Errorf("init manager not initialized")
	}

	statePath := filepath.Join(im.config.DataDir, "raft_state.json")

	raftState.mu.Lock()
	state := struct {
		CurrentTerm int64      `json:"current_term"`
		VotedFor    int64      `json:"voted_for"`
		Logs        []LogEntry `json:"logs"`
	}{
		CurrentTerm: raftState.currentTerm,
		VotedFor:    raftState.votedFor,
		Logs:        raftState.logs,
	}
	raftState.mu.Unlock()

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

// 在服务启动时调用此函数初始化资源
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
	//gatewayClient := client.GetGatewayClient()
	////todo  config
	//gatewayClient.RegisterSever(context.Background(), &gateway.RegisterSeverReq{
	//	ServerHost: "",
	//})
	log.Log.Info("raft state initialized")
	return nil
}
