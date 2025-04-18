package configs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/google/uuid"
)

// Config 配置结构体 - 作为整个系统的单一配置源
type Config struct {
	// 节点配置
	NodeID      *int64   `json:"node_id"`      // 节点ID
	ClusterID   int64    `json:"cluster_id"`   // 集群ID
	Peers       []string `json:"peers"`        // 集群中所有节点的地址
	Port        string   `json:"port"`         // 服务端口号
	GatewayHost string   `json:"gateway_host"` // 网关地址
	IsMaster    bool     `json:"is_master"`    // 自己是否是master
	MasterAddr  string   `json:"master_addr"`  // master 地址

	// Raft配置
	ElectionTimeout int    `json:"election_timeout"` // 选举超时时间（毫秒）
	HeartbeatPeriod int    `json:"heartbeat_period"` // 心跳周期（毫秒）
	DataDir         string `json:"data_dir"`         // 数据目录
	SnapshotCount   int64  `json:"snapshot_count"`   // 触发快照的日志条目数

	// 日志配置
	LogLevel string `json:"log_level"` // 日志级别
	LogDir   string `json:"log_dir"`   // 日志目录

	// 其他配置
	MetricsEnabled bool `json:"metrics_enabled"` // 是否启用指标收集
}

var (
	globalConfig *Config
	configLock   sync.RWMutex
	loaded       bool
)

// LoadConfig 从文件加载配置
func LoadConfig(configPath string) error {
	configLock.Lock()
	defer configLock.Unlock()

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %v", err)
	}

	// 设置默认值
	if config.NodeID == nil {
		config.NodeID = thrift.Int64Ptr(int64(uuid.New().ID()))
	}

	if config.DataDir == "" {
		config.DataDir = "data"
	}

	if config.LogLevel == "" {
		config.LogLevel = "info"
	}

	if config.LogDir == "" {
		config.LogDir = "logs"
	}

	if config.ElectionTimeout == 0 {
		config.ElectionTimeout = 1000
	}

	if config.HeartbeatPeriod == 0 {
		config.HeartbeatPeriod = 100
	}

	if config.SnapshotCount == 0 {
		config.SnapshotCount = 10000
	}

	// 确保数据目录存在
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// 确保日志目录存在
	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	globalConfig = &config
	loaded = true

	return nil
}

// GetConfig 获取配置
func GetConfig() *Config {
	configLock.RLock()
	defer configLock.RUnlock()

	if !loaded {
		panic("Config not loaded")
	}

	// 返回一个配置副本以防止修改
	configCopy := *globalConfig
	return &configCopy
}

// UpdatePeers 更新集群节点列表
func UpdatePeers(peers []string) {
	configLock.Lock()
	defer configLock.Unlock()

	if !loaded {
		panic("Config not loaded")
	}

	globalConfig.Peers = peers
}

// SaveConfig 保存配置到文件
func SaveConfig(configPath string) error {
	configLock.RLock()
	defer configLock.RUnlock()

	if !loaded {
		return fmt.Errorf("config not loaded")
	}

	data, err := json.MarshalIndent(globalConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// ToRaftConfig 将Config转换为infra.RaftConfig
func (c *Config) ToRaftConfig() interface{} {
	// 注意：这个返回类型为interface{}是为了避免循环导入
	// 实际使用时，调用方需要将其断言为infra.RaftConfig
	return struct {
		NodeId          *int64
		ClusterId       int64
		Peers           []string
		ElectionTimeout int
		HeartbeatPeriod int
		DataDir         string
		SnapshotCount   int64
		Port            string
		GatewayHost     string
		IsMaster        bool
		MasterAddr      string
	}{
		NodeId:          c.NodeID,
		ClusterId:       c.ClusterID,
		Peers:           c.Peers,
		ElectionTimeout: c.ElectionTimeout,
		HeartbeatPeriod: c.HeartbeatPeriod,
		DataDir:         c.DataDir,
		SnapshotCount:   c.SnapshotCount,
		Port:            c.Port,
		GatewayHost:     c.GatewayHost,
		IsMaster:        c.IsMaster,
		MasterAddr:      c.MasterAddr,
	}
}

// 获取节点ID
func GetNodeID() int64 {
	cfg := GetConfig()
	if cfg.NodeID != nil {
		return *cfg.NodeID
	}
	return 0
}

// 是否为主节点
func IsMaster() bool {
	return GetConfig().IsMaster
}

// 获取集群节点列表
func GetPeers() []string {
	return GetConfig().Peers
}

// 获取本节点服务端口
func GetPort() string {
	return GetConfig().Port
}

// 获取网关地址
func GetGatewayHost() string {
	return GetConfig().GatewayHost
}

// 获取数据目录
func GetDataDir() string {
	return GetConfig().DataDir
}

// 获取主节点地址
func GetMasterAddr() string {
	return GetConfig().MasterAddr
}

// 设置主节点地址
func SetMasterAddr(addr string) {
	configLock.Lock()
	defer configLock.Unlock()

	if !loaded {
		panic("Config not loaded")
	}

	globalConfig.MasterAddr = addr
}

// 设置节点ID
func SetNodeID(id int64) {
	configLock.Lock()
	defer configLock.Unlock()

	if !loaded {
		panic("Config not loaded")
	}

	globalConfig.NodeID = thrift.Int64Ptr(id)
}
