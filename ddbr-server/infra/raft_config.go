package infra

// RaftConfig 表示Raft节点配置
type RaftConfig struct {
	NodeId          int64    `json:"node_id"`          // 节点ID
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
