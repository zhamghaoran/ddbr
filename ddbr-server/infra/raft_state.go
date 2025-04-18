package infra

import (
	"context"
	"github.com/google/uuid"
	"sync"
	"time"
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
)

// RaftState 表示Raft节点的状态
type RaftState struct {
	Mu          sync.Mutex // 用于保护并发访问
	CurrentTerm int64      // 当前任期
	VotedFor    int64      // 当前任期投票给的候选人ID，如果没有则为-1
	Logs        []LogEntry // 日志条目
	nodeId      int64      // 当前节点ID
}

// LogEntry 表示日志条目
type LogEntry struct {
	Term    int64  // 写入该日志时的任期
	Index   int64  // 日志索引
	Command string // 日志命令内容
}

// 全局Raft状态实例
var (
	raftState = &RaftState{
		CurrentTerm: 0,
		VotedFor:    -1,
		Logs:        []LogEntry{},
		nodeId:      int64(uuid.New().ID()), // 默认节点ID，实际使用时应该从配置中读取
	}
	raftStateOnce sync.Once
)

// GetRaftState 获取Raft状态实例
func GetRaftState() *RaftState {
	raftStateOnce.Do(func() {
		// 确保只初始化一次
		if raftState == nil {
			raftState = &RaftState{
				CurrentTerm: 0,
				VotedFor:    -1,
				Logs:        []LogEntry{},
				nodeId:      int64(uuid.New().ID()),
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
	return rs.nodeId
}

// SetNodeId 设置节点ID
func (rs *RaftState) SetNodeId(nodeId int64) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	rs.nodeId = nodeId
}
func (rs *RaftState) BeginVote() {

}
func (rs *RaftState) ExpiredTimer(ctx context.Context, masterAddr string, closeCh chan int) {
	serverClient := client.GetLeaderClient(masterAddr)
	selfSpinTime := GetServerConfig().ElectionTimeout
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
			SetSetPeers(resp.GetPeers())
		}
	}

}
