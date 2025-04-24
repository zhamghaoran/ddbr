package infra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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
		return fmt.Errorf("加载配置文件失败: %v", err)
	}
	// 获取配置并设置master标志
	config := configs.GetConfig()
	configs.SetIsMaster(master)

	// 更新数据路径
	im.dataPath = config.DataDir

	// 确保数据目录存在
	if err := os.MkdirAll(im.dataPath, 0755); err != nil {
		return fmt.Errorf("创建数据目录 '%s' 失败: %v", im.dataPath, err)
	}

	// 更新RaftState - 不使用类型断言，直接从config构建RaftConfig
	raftConfig := RaftConfig{
		NodeId:          config.NodeID,
		ClusterId:       config.ClusterID,
		Peers:           config.Peers,
		ElectionTimeout: config.ElectionTimeout,
		HeartbeatPeriod: config.HeartbeatPeriod,
		DataDir:         config.DataDir,
		SnapshotCount:   config.SnapshotCount,
		Port:            config.Port,
		GatewayHost:     config.GatewayHost,
		IsMaster:        config.IsMaster,
		MasterAddr:      config.MasterAddr,
	}
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

	log.Log.Infof("尝试从 %s 加载持久化状态", statePath)
	data, err := ioutil.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Log.Infof("状态文件不存在，从零开始")
			return nil
		}
		return err
	}

	type persistentState struct {
		CurrentTerm int64             `json:"current_term"`
		VotedFor    int64             `json:"voted_for"`
		Logs        []*sever.LogEntry `json:"logs"`
	}

	var state persistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state file: %v", err)
	}

	raftState.SetCurrentTerm(state.CurrentTerm)
	raftState.SetVotedFor(state.VotedFor)
	raftState.SetLogs(state.Logs)

	// 在leader节点，将加载的日志应用到状态机
	if configs.IsMaster() && len(state.Logs) > 0 {
		log.Log.Infof("开始应用 %d 条日志到状态机", len(state.Logs))
		lastApplied := raftState.GetLastApplied()

		for _, entry := range state.Logs {
			// 只应用尚未应用的日志
			if entry.Index > lastApplied {
				result, err := ApplyLogToStateMachine(*entry)
				if err != nil {
					log.Log.Warnf("应用日志条目 %d 失败: %v", entry.Index, err)
					continue
				}
				log.Log.Infof("应用日志条目 %d 成功: %s → %s", entry.Index, entry.Command, result)

				// 更新lastApplied
				raftState.SetLastApplied(entry.Index)
			}
		}

		// 更新commitIndex
		if len(state.Logs) > 0 {
			lastIndex := state.Logs[len(state.Logs)-1].Index
			raftState.SetCommitIndex(lastIndex)
			log.Log.Infof("更新commit索引为 %d", lastIndex)
		}
	}

	log.Log.Infof("从存储中恢复了 %d 条日志记录", len(state.Logs))
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
		CurrentTerm int64             `json:"current_term"`
		VotedFor    int64             `json:"voted_for"`
		Logs        []*sever.LogEntry `json:"logs"`
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
func (im *InitManager) JoinCluster() error {
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
			err := SyncLogsWithLeader(ctx, resp.LeaderId)
			if err != nil {
				log.Log.Errorf("failed to sync logs: %v", err)
				return
			}
		}()
		im.SetJoinedCluster(true)
		log.Log.Infof("Successfully joined the cluster, leader: %d", resp.LeaderId)
	}

	return nil
}

// InitializeResources 初始化所有资源
func InitializeResources(configPath string, master bool) error {
	im := GetInitManager()
	if configPath == "" {
		return fmt.Errorf("configPath is empty")
	}
	if err := im.LoadConfig(configPath, master); err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}
	// 初始化Raft状态
	if err := im.InitializeRaftState(); err != nil {
		return fmt.Errorf("failed to initialize Raft state: %v", err)
	}

	// 向网关注册并加入集群
	if err := im.JoinCluster(); err != nil {
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
	log.Log.Infof("获取到网关响应: %+v", *resp)
	if err != nil {
		log.Log.Errorf("Failed to register sever: %v", err)
		return nil, err
	}

	// 两种情况：
	// 1. 如果当前节点是master，向网关注册成为leader
	// 2. 如果当前节点不是master，且gateway已经有leader，则向leader注册
	if config.IsMaster {
		// 只有当从网关获取的leader ID为-1（即还没有leader）时，才设置自己为leader
		if resp.LeaderId == -1 || resp.LeaderId == 0 {
			log.Log.Infof("准备注册为leader节点，当前leaderId=%d", resp.LeaderId)

			setResp, err := gatewayClient.SetLeader(ctx, &gateway.SetLeaderReq{})
			if err != nil {
				// 如果错误消息包含"leader already exists"，说明同时有其他节点也在注册为leader
				if strings.Contains(err.Error(), "leader already exists") {
					log.Log.Warnf("发现已存在leader，切换为follower模式: %v", err)
					configs.SetIsMaster(false)
				} else {
					log.Log.Errorf("注册leader失败: %v", err)
					return resp, err
				}
			} else {
				log.Log.Infof("成功注册为leader节点，master地址: %s", setResp.GetMasterHost())
			}
		} else {
			log.Log.Warnf("当前节点配置为master但已存在leader (ID: %d)，以follower模式运行", resp.LeaderId)
			configs.SetIsMaster(false)
		}
	}

	// 如果不是master并且网关返回了leader地址，向leader注册
	if !configs.IsMaster() && resp.LeaderHost != "" {
		// 更新主节点地址
		configs.SetMasterAddr(resp.LeaderHost)

		masterClient := client.GetLeaderClient(resp.LeaderHost)
		if masterClient != nil {
			_, err := masterClient.JoinCluster(ctx, &sever.JoinClusterReq{})
			if err != nil {
				log.Log.Errorf("加入集群失败: %v", err)
				return resp, err
			}
			log.Log.Infof("follower join cluster, leader host: %v", resp.LeaderHost)
		}
	}
	return resp, nil
}
