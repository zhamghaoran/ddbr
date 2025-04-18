package infra

import (
	"context"
	"sync"
	"time"
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
)

// LogEntry 表示日志条目
type LogEntry struct {
	Term    int64  // 写入该日志时的任期
	Index   int64  // 日志索引
	Command string // 日志命令内容
}

// RaftState 表示Raft节点的状态和配置
type RaftState struct {
	// 状态相关字段
	Mu          sync.Mutex // 用于保护并发访问
	CurrentTerm int64      // 当前任期
	VotedFor    int64      // 当前任期投票给的候选人ID，如果没有则为-1
	Logs        []LogEntry // 日志条目

	// 配置相关字段 (从RaftConfig整合而来)
	NodeId          int64    // 节点ID (不再使用指针)
	ClusterId       int64    // 集群ID
	Peers           []string // 集群中所有节点的地址
	ElectionTimeout int      // 选举超时时间（毫秒）
	HeartbeatPeriod int      // 心跳周期（毫秒）
	DataDir         string   // 数据目录
	SnapshotCount   int64    // 触发快照的日志条目数
	Port            string   // 服务端口号
	GatewayHost     string   // 网关地址
	IsMaster        bool     // 自己是否是master
	MasterAddr      string   // master 地址
}

// 全局Raft状态实例
var (
	raftState     *RaftState
	raftStateOnce sync.Once
)

// GetRaftState 获取Raft状态实例
func GetRaftState() *RaftState {
	raftStateOnce.Do(func() {
		// 确保只初始化一次
		if raftState == nil {
			raftState = &RaftState{
				// 状态默认值
				CurrentTerm: 0,
				VotedFor:    -1,
				Logs:        []LogEntry{},

				// 配置默认值
				NodeId:          0, // 将在初始化时设置
				ClusterId:       1,
				ElectionTimeout: 1000,
				HeartbeatPeriod: 100,
				DataDir:         "data",
				SnapshotCount:   10000,
			}
		}
	})
	return raftState
}

// GetCurrentTerm 获取当前任期
func (rs *RaftState) GetCurrentTerm() int64 {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	return rs.CurrentTerm
}

// SetCurrentTerm 设置当前任期
func (rs *RaftState) SetCurrentTerm(term int64) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	rs.CurrentTerm = term
}

// GetVotedFor 获取投票对象
func (rs *RaftState) GetVotedFor() int64 {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	return rs.VotedFor
}

// SetVotedFor 设置投票对象
func (rs *RaftState) SetVotedFor(candidateId int64) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	rs.VotedFor = candidateId
}

// GetLogs 获取日志条目
func (rs *RaftState) GetLogs() []LogEntry {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	logsCopy := make([]LogEntry, len(rs.Logs))
	copy(logsCopy, rs.Logs)
	return logsCopy
}

// SetLogs 设置日志条目
func (rs *RaftState) SetLogs(logs []LogEntry) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	rs.Logs = logs
}

// GetNodeId 获取节点ID
func (rs *RaftState) GetNodeId() int64 {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	return rs.NodeId
}

// SetNodeId 设置节点ID
func (rs *RaftState) SetNodeId(nodeId int64) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	rs.NodeId = nodeId
}

// GetConfig 获取完整配置
func (rs *RaftState) GetConfig() RaftConfig {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	return RaftConfig{
		NodeId:          &rs.NodeId,
		ClusterId:       rs.ClusterId,
		Peers:           rs.Peers,
		ElectionTimeout: rs.ElectionTimeout,
		HeartbeatPeriod: rs.HeartbeatPeriod,
		DataDir:         rs.DataDir,
		SnapshotCount:   rs.SnapshotCount,
		Port:            rs.Port,
		GatewayHost:     rs.GatewayHost,
		IsMaster:        rs.IsMaster,
		MasterAddr:      rs.MasterAddr,
	}
}

// UpdateConfig 使用RaftConfig更新配置
func (rs *RaftState) UpdateConfig(config RaftConfig) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()

	if config.NodeId != nil {
		rs.NodeId = *config.NodeId
	}
	rs.ClusterId = config.ClusterId
	rs.Peers = config.Peers
	rs.ElectionTimeout = config.ElectionTimeout
	rs.HeartbeatPeriod = config.HeartbeatPeriod
	rs.DataDir = config.DataDir
	rs.SnapshotCount = config.SnapshotCount
	rs.Port = config.Port
	rs.GatewayHost = config.GatewayHost
	rs.IsMaster = config.IsMaster
	rs.MasterAddr = config.MasterAddr
}

// SetPeers 设置集群节点列表
func (rs *RaftState) SetPeers(peers []string) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	rs.Peers = peers
}

// BeginVote 开始选举
func (rs *RaftState) BeginVote() {
	// 选举逻辑实现
}

// ExpiredTimer 心跳超时检测
func (rs *RaftState) ExpiredTimer(ctx context.Context, masterAddr string, closeCh chan int) {
	serverClient := client.GetLeaderClient(masterAddr)
	selfSpinTime := rs.ElectionTimeout / 1000 // 转换为秒
	for {
		select {
		case <-closeCh:
			return
		case <-time.After(time.Second * time.Duration(selfSpinTime)):
			resp, err := serverClient.HeartBeat(ctx, &sever.HeartbeatReq{})
			if err != nil {
				rs.BeginVote()
				return
			}
			rs.SetPeers(resp.GetPeers())
		}
	}
}
