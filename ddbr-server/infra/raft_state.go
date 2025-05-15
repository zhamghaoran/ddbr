package infra

import (
	"context"
	"strings"
	"sync"
	"time"
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

// RaftState 表示Raft节点的状态和配置
type RaftState struct {
	// 状态相关字段
	Mu          sync.Mutex        // 用于保护并发访问
	CurrentTerm int64             // 当前任期
	VotedFor    int64             // 当前任期投票给的候选人ID，如果没有则为-1
	Logs        []*sever.LogEntry // 日志条目

	// 日志提交相关字段
	CommitIndex int64 // 已知已提交的最高日志条目索引
	LastApplied int64 // 已应用到状态机的最高日志条目索引

	// 仅Leader使用的字段
	NextIndex  map[string]int64 // 对于每个服务器，要发送的下一个日志条目索引
	MatchIndex map[string]int64 // 对于每个服务器，已知已复制的最高日志条目

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
				Logs:        []*sever.LogEntry{},

				// 日志提交相关字段
				CommitIndex: 0,
				LastApplied: 0,

				// 仅Leader使用的字段
				NextIndex:  make(map[string]int64),
				MatchIndex: make(map[string]int64),

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
func (rs *RaftState) GetLogs() []*sever.LogEntry {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	logsCopy := make([]*sever.LogEntry, len(rs.Logs))
	copy(logsCopy, rs.Logs)
	return logsCopy
}

// SetLogs 设置日志条目
func (rs *RaftState) SetLogs(logs []*sever.LogEntry) {
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

// GetCommitIndex 获取已提交的最高日志索引
func (rs *RaftState) GetCommitIndex() int64 {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	return rs.CommitIndex
}

// SetCommitIndex 设置已提交的最高日志索引
func (rs *RaftState) SetCommitIndex(index int64) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	rs.CommitIndex = index
}

// GetLastApplied 获取已应用到状态机的最高日志索引
func (rs *RaftState) GetLastApplied() int64 {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	return rs.LastApplied
}

// SetLastApplied 设置已应用到状态机的最高日志索引
func (rs *RaftState) SetLastApplied(index int64) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	rs.LastApplied = index
}

// UpdateCommitIndex 根据多数节点的复制情况更新commitIndex (仅Leader调用)
func (rs *RaftState) UpdateCommitIndex() int64 {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()

	// 非Leader不应该更新CommitIndex
	if !rs.IsMaster {
		return rs.CommitIndex
	}

	// 获取当前日志长度
	logLength := int64(len(rs.Logs))
	if logLength == 0 {
		return rs.CommitIndex
	}

	// 统计每个日志索引被复制到多少节点
	counts := make(map[int64]int)
	counts[0] = 1 // 自己的日志

	// 计算每个节点已复制的日志数量
	for _, matchIndex := range rs.MatchIndex {
		if matchIndex > 0 {
			counts[matchIndex]++
		}
	}

	// 集群节点总数（包括自己）
	totalNodes := len(rs.Peers) + 1
	majority := totalNodes/2 + 1

	// 检查哪些日志索引已经被大多数节点接收
	newCommitIndex := rs.CommitIndex
	for idx := rs.CommitIndex + 1; idx < logLength; idx++ {
		// 如果当前任期的日志已被多数节点复制，则可以提交
		// 注意：根据Raft论文，只有当前任期的日志才能通过计数提交
		if counts[idx] >= majority && rs.Logs[idx].Term == rs.CurrentTerm {
			newCommitIndex = idx
		}
	}

	// 更新commitIndex
	if newCommitIndex > rs.CommitIndex {
		rs.CommitIndex = newCommitIndex
		ctx := context.Background()
		log.Log.CtxInfof(ctx, "Updated commitIndex to %d", newCommitIndex)
	}

	return rs.CommitIndex
}

// ApplyCommittedLogs 应用已提交但未应用的日志到状态机
func (rs *RaftState) ApplyCommittedLogs() error {
	rs.Mu.Lock()

	// 检查是否有新的已提交日志需要应用
	if rs.LastApplied >= rs.CommitIndex {
		rs.Mu.Unlock()
		return nil
	}

	// 获取需要应用的日志条目
	logsToApply := make([]*sever.LogEntry, 0)
	for i := rs.LastApplied + 1; i <= rs.CommitIndex && i < int64(len(rs.Logs)); i++ {
		logsToApply = append(logsToApply, rs.Logs[i])
	}

	// 如果没有要应用的日志，返回
	if len(logsToApply) == 0 {
		rs.Mu.Unlock()
		return nil
	}

	// 释放锁，避免长时间持有锁
	rs.Mu.Unlock()

	// 应用日志到状态机
	ctx := context.Background()
	for _, entry := range logsToApply {
		result, err := ApplyLogToStateMachine(*entry)
		if err != nil {
			// 如果是日志已应用的错误，记录并继续，不影响后续日志的应用
			if strings.Contains(err.Error(), "log entry already applied") {
				log.Log.CtxWarnf(ctx, "Log entry %d already applied, skipping: %v", entry.Index, err)
				// 更新LastApplied以跳过此条目
				rs.SetLastApplied(entry.Index)
				continue
			}
			// 其他错误则返回
			log.Log.CtxErrorf(ctx, "Failed to apply log entry %d: %v", entry.Index, err)
			return err
		}
		log.Log.CtxInfof(ctx, "Applied log entry %d to state machine: %s", entry.Index, result)

		// 更新lastApplied
		rs.SetLastApplied(entry.Index)
	}

	return nil
}

// GetLastLogIndex 获取最后一个日志的索引
func (rs *RaftState) GetLastLogIndex() int64 {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()

	if len(rs.Logs) == 0 {
		return 0
	}

	// 返回最后一个日志条目的索引，确保与日志条目中的Index字段一致
	// 而不是直接使用切片索引
	return rs.Logs[len(rs.Logs)-1].Index
}

// AppendLog 追加单个日志条目到日志列表
func (rs *RaftState) AppendLog(entry *sever.LogEntry) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()

	ctx := context.Background()
	log.Log.CtxInfof(ctx, "正在追加新日志条目: {Index: %d, Term: %d, Command: %s}",
		entry.Index, entry.Term, entry.Command)

	// 检查Index是否有效
	if entry.Index < 1 {
		log.Log.CtxWarnf(ctx, "追加的日志条目索引无效 (< 1): %d", entry.Index)
		entry.Index = 1
		if len(rs.Logs) > 0 {
			entry.Index = rs.Logs[len(rs.Logs)-1].Index + 1
		}
		log.Log.CtxInfof(ctx, "调整日志条目索引为: %d", entry.Index)
	}

	// 检查日志是否存在冲突
	for i, e := range rs.Logs {
		if e.Index == entry.Index {
			if e.Term != entry.Term {
				log.Log.CtxWarnf(ctx, "检测到冲突的日志条目，截断后追加，位置: %d", i)
				// 截断日志
				rs.Logs = rs.Logs[:i]
				break
			} else {
				// 索引和任期都相同，可能是重复操作，跳过
				log.Log.CtxInfof(ctx, "忽略重复的日志条目: {Index: %d, Term: %d}",
					entry.Index, entry.Term)
				return
			}
		}
	}

	// 追加日志
	rs.Logs = append(rs.Logs, entry)
	log.Log.CtxInfof(ctx, "追加日志条目成功，当前日志长度: %d", len(rs.Logs))
}
