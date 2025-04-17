package service

import (
	"sync"

	"github.com/google/uuid"
)

// RaftState 表示Raft节点的状态
type RaftState struct {
	mu          sync.Mutex // 用于保护并发访问
	currentTerm int64      // 当前任期
	votedFor    int64      // 当前任期投票给的候选人ID，如果没有则为-1
	logs        []LogEntry // 日志条目
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
		currentTerm: 0,
		votedFor:    -1,
		logs:        []LogEntry{},
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
				currentTerm: 0,
				votedFor:    -1,
				logs:        []LogEntry{},
				nodeId:      int64(uuid.New().ID()),
			}
		}
	})
	return raftState
}

// GetCurrentTerm 获取当前任期
func (rs *RaftState) GetCurrentTerm() int64 {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.currentTerm
}

// SetCurrentTerm 设置当前任期
func (rs *RaftState) SetCurrentTerm(term int64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.currentTerm = term
}

// GetVotedFor 获取投票对象
func (rs *RaftState) GetVotedFor() int64 {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.votedFor
}

// SetVotedFor 设置投票对象
func (rs *RaftState) SetVotedFor(candidateId int64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.votedFor = candidateId
}

// GetLogs 获取日志条目
func (rs *RaftState) GetLogs() []LogEntry {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	logsCopy := make([]LogEntry, len(rs.logs))
	copy(logsCopy, rs.logs)
	return logsCopy
}

// SetLogs 设置日志条目
func (rs *RaftState) SetLogs(logs []LogEntry) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.logs = logs
}

// GetNodeId 获取节点ID
func (rs *RaftState) GetNodeId() int64 {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.nodeId
}

// SetNodeId 设置节点ID
func (rs *RaftState) SetNodeId(nodeId int64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.nodeId = nodeId
}
