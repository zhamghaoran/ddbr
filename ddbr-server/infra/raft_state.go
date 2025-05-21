package infra

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"time"
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/configs"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"

	"github.com/cloudwego/kitex/client/callopt"
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
	ctx := context.Background()
	log.Log.CtxInfof(ctx, "开始选举：节点 %d 参与选举", rs.NodeId)

	// 在函数作用域定义变量
	var voteReq *sever.RequestVoteReq
	var peers []string
	var currentTerm int64
	var lastLogIndex, lastLogTerm int64
	var noOtherNodes bool

	// 在一个独立的作用域中使用锁，确保锁会被释放
	func() {
		rs.Mu.Lock()
		defer rs.Mu.Unlock() // 确保无论如何锁都会被释放

		// 检查当前配置情况
		peerCount := len(rs.Peers)
		log.Log.CtxInfof(ctx, "当前节点配置：节点ID=%d, 集群节点数=%d, 节点列表=%v",
			rs.NodeId, peerCount, rs.Peers)

		if peerCount == 0 {
			log.Log.CtxWarnf(ctx, "没有其他节点，单节点模式，直接成为Leader")
			rs.CurrentTerm++
			rs.VotedFor = rs.NodeId
			// 标记没有其他节点，稍后在锁外成为Leader
			noOtherNodes = true
			return
		}

		// 1. 增加当前任期
		rs.CurrentTerm++
		currentTerm = rs.CurrentTerm

		// 2. 投票给自己
		rs.VotedFor = rs.NodeId

		// 3. 重置选举计时器 (在ExpiredTimer中已处理)

		// 4. 获取自己的最后日志信息
		lastLogIndex = 0
		lastLogTerm = 0
		if len(rs.Logs) > 0 {
			lastLog := rs.Logs[len(rs.Logs)-1]
			lastLogIndex = lastLog.Index
			lastLogTerm = lastLog.Term
		}

		// 5. 准备请求投票请求 - 在锁中构建请求
		voteReq = &sever.RequestVoteReq{
			Term:         currentTerm,
			CandidateId:  rs.NodeId,
			LastLogIndex: lastLogIndex,
			LastLogTerm:  lastLogTerm,
		}

		// 复制一份peers以避免锁争用
		peers = make([]string, len(rs.Peers))
		copy(peers, rs.Peers)
	}() // 锁在这里被释放

	// 如果没有其他节点，在锁外成为Leader
	if noOtherNodes {
		log.Log.CtxInfof(ctx, "没有其他节点，成为Leader")
		rs.becomeLeaderUnlocked(ctx)
		return
	}

	// 检查如果没有peers，直接成为Leader
	if len(peers) == 0 {
		log.Log.CtxInfof(ctx, "没有其他节点，成为Leader")
		rs.becomeLeaderUnlocked(ctx)
		return
	}

	log.Log.CtxInfof(ctx, "选举：节点 %d 增加任期，准备发送投票请求: %+v", rs.NodeId, voteReq)

	// 6. 向所有其他节点发送RequestVote RPC
	// 初始化选票统计（包括自己的一票）
	votesReceived := 1
	votesNeeded := len(peers)/2 + 1

	log.Log.CtxInfof(ctx, "选举：节点 %d 需要获得 %d 票才能当选 (包括自己的一票)", rs.NodeId, votesNeeded)

	if votesNeeded <= 1 {
		log.Log.CtxInfof(ctx, "节点 %d 只需要自己的一票就能当选，直接成为Leader", rs.NodeId)
		rs.becomeLeaderUnlocked(ctx)
		return
	}

	// 创建等待组，用于并行发送投票请求
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 向每个Peer发送请求
	for i, peer := range peers {
		if peer == "" {
			log.Log.CtxWarnf(ctx, "跳过空地址节点 %d", i)
			continue
		}

		// 检查是否是自己的地址，跳过向自己发送投票请求
		peerHost := peer
		if !strings.Contains(peerHost, ":") {
			port := rs.Port
			if port == "" {
				port = "8080" // 默认端口
			}
			peerHost = peerHost + ":" + port
		}

		// 获取当前节点地址
		selfAddr := configs.GetConfig().MasterAddr
		// 如果当前节点地址为空，则使用IP+Port构建
		if selfAddr == "" {
			// 使用IP地址和端口构建
			if rs.Port != "" {
				selfAddr = "127.0.0.1:" + rs.Port // 本地地址
			}
		}

		wg.Add(1)
		go func(idx int, peerAddr string) {
			// 使用defer确保wg.Done()一定会被调用
			defer func() {
				if r := recover(); r != nil {
					log.Log.CtxErrorf(ctx, "选举请求处理发生panic: %v", r)
				}
				wg.Done()
			}()

			log.Log.CtxInfof(ctx, "选举：节点 %d 向节点[%d] %s 发送投票请求", rs.NodeId, idx, peerAddr)

			// 不要在这里手动添加端口，让GetLeaderClient处理
			log.Log.CtxInfof(ctx, "peerAddr is %s", peerAddr)
			rpcClient := client.GetLeaderClient(peerAddr)
			if rpcClient == nil {
				log.Log.CtxWarnf(ctx, "选举：获取节点 %s 的客户端失败", peerAddr)
				return
			}

			// 发送请求
			log.Log.CtxInfof(ctx, "选举：向节点 %s 发送RPC请求：%+v", peerAddr, voteReq)
			voteResp, err := rpcClient.RequestVote(ctx, voteReq, callopt.WithConnectTimeout(time.Second*2))
			if err != nil {
				log.Log.CtxErrorf(ctx, "选举：向节点 %s 发送投票请求失败: %v", peerAddr, err)
				return
			}

			log.Log.CtxInfof(ctx, "选举：收到节点 %s 的投票响应: %+v", peerAddr, voteResp)

			// 处理响应 - 使用局部锁减少锁冲突
			func() {
				rs.Mu.Lock()
				defer rs.Mu.Unlock()

				// 如果在等待响应期间任期已经变更，放弃这次选举
				if voteReq.Term != rs.CurrentTerm {
					log.Log.CtxInfof(ctx, "选举：节点 %d 任期已变更 %d -> %d，放弃当前选举",
						rs.NodeId, voteReq.Term, rs.CurrentTerm)
					return
				}

				// 如果收到的响应中任期更高，更新自己的任期并转为Follower
				if voteResp.Term > rs.CurrentTerm {
					log.Log.CtxInfof(ctx, "选举：节点 %d 收到更高任期 %d > %d，转为Follower",
						rs.NodeId, voteResp.Term, rs.CurrentTerm)
					rs.CurrentTerm = voteResp.Term
					rs.VotedFor = -1
					rs.IsMaster = false
					return
				}
			}()

			// 如果获得选票
			if voteResp.VoteGranted {
				mu.Lock()
				votesReceived++
				receivedSoFar := votesReceived
				mu.Unlock()

				log.Log.CtxInfof(ctx, "选举：节点 %d 获得节点 %s 的选票，当前得票 %d/%d",
					rs.NodeId, peerAddr, receivedSoFar, votesNeeded+len(peers)-1)

				// 如果获得多数选票，成为Leader
				if receivedSoFar >= votesNeeded {
					log.Log.CtxInfof(ctx, "选举：节点 %d 获得多数选票 (%d/%d)，当选为Leader！",
						rs.NodeId, receivedSoFar, votesNeeded+len(peers)-1)
					rs.becomeLeaderUnlocked(ctx)
				}
			} else {
				log.Log.CtxInfof(ctx, "选举：节点 %d 未获得节点 %s 的选票，原因: %s",
					rs.NodeId, peerAddr, voteResp.Common.Message)
			}
		}(i, peer)
	}

	// 等待所有RPC完成或超时
	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	// 选举超时时间
	select {
	case <-wgDone:
		log.Log.CtxInfof(ctx, "选举：节点 %d 完成所有投票请求处理", rs.NodeId)
	case <-time.After(time.Millisecond * time.Duration(rs.ElectionTimeout+2000)):
		log.Log.CtxWarnf(ctx, "选举：节点 %d 选举超时", rs.NodeId)
	}

	// 最终检查选举结果
	mu.Lock()
	finalVotes := votesReceived
	mu.Unlock()
	log.Log.CtxInfof(ctx, "选举：节点 %d 最终得票 %d/%d，需要 %d 票才能当选",
		rs.NodeId, finalVotes, len(peers)+1, votesNeeded)
}

// becomeLeaderUnlocked 是becomeLeader的新版本，不依赖于外部锁的状态
func (rs *RaftState) becomeLeaderUnlocked(ctx context.Context) {
	// 获取锁
	rs.Mu.Lock()
	defer rs.Mu.Unlock()

	// 验证状态，确保我们仍能成为Leader
	if rs.IsMaster {
		log.Log.CtxInfof(ctx, "节点 %d 已经是Leader，无需再次成为Leader", rs.NodeId)
		return
	}

	// 初始化Leader状态
	rs.IsMaster = true

	// 初始化nextIndex和matchIndex
	for _, peer := range rs.Peers {
		// 对于每个服务器，nextIndex初始化为Leader最后一条日志索引+1
		lastLogIndex := int64(0)
		if len(rs.Logs) > 0 {
			lastLogIndex = rs.Logs[len(rs.Logs)-1].Index
		}
		rs.NextIndex[peer] = lastLogIndex + 1

		// matchIndex初始化为0
		rs.MatchIndex[peer] = 0
	}

	// 当前任期
	currentTerm := rs.CurrentTerm

	log.Log.CtxInfof(ctx, "选举成功：节点 %d 成为Leader，任期 %d, 准备发送心跳",
		rs.NodeId, currentTerm)

	// 更新配置，确保全局状态一致
	configs.SetIsMaster(true)

	// 释放锁之后启动心跳循环
	go rs.heartbeatLoop(ctx)
}

// becomeLeader 保留原有函数名，但现在委托给becomeLeaderUnlocked
func (rs *RaftState) becomeLeader() {
	rs.becomeLeaderUnlocked(context.Background())
}

// heartbeatLoop 定期发送心跳的循环
func (rs *RaftState) heartbeatLoop(ctx context.Context) {
	log.Log.CtxInfof(ctx, "Leader %d 启动定期心跳发送", rs.NodeId)

	// 创建心跳定时器，初始立即发送一次心跳
	heartbeatTicker := time.NewTicker(time.Millisecond * time.Duration(rs.HeartbeatPeriod))
	defer heartbeatTicker.Stop()

	// 立即发送第一次心跳
	rs.sendHeartbeats(ctx)

	for {
		select {
		case <-heartbeatTicker.C:
			// 检查是否仍然是Leader
			rs.Mu.Lock()
			isStillLeader := rs.IsMaster
			rs.Mu.Unlock()

			if !isStillLeader {
				log.Log.CtxInfof(ctx, "节点 %d 不再是Leader，停止心跳发送", rs.NodeId)
				return
			}

			// 发送心跳
			rs.sendHeartbeats(ctx)
		}
	}
}

// sendHeartbeats 发送心跳给所有Follower
func (rs *RaftState) sendHeartbeats(ctx context.Context) {
	log.Log.CtxInfof(ctx, "Leader %d 开始发送心跳，当前集群节点数: %d", rs.NodeId, len(rs.Peers))

	if len(rs.Peers) == 0 {
		log.Log.CtxInfof(ctx, "Leader %d 没有其他节点，跳过心跳发送", rs.NodeId)
		return
	}

	// 向每个Follower发送空AppendEntries RPC
	successCount := 0
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, peer := range rs.Peers {
		if peer == "" {
			log.Log.CtxWarnf(ctx, "跳过空地址节点 %d", i)
			continue
		}

		wg.Add(1)
		go func(idx int, peerAddr string) {
			defer wg.Done()

			rs.Mu.Lock()
			// 再次检查是否仍是Leader
			if !rs.IsMaster {
				rs.Mu.Unlock()
				return
			}

			// 准备心跳请求
			heartbeatReq := &sever.AppendEntriesReq{
				Term:         rs.CurrentTerm,
				LeaderId:     rs.NodeId,
				PrevLogIndex: 0, // 心跳可以简化这些字段
				PrevLogTerm:  0,
				Entries:      []string{}, // 空日志条目
				LeaderCommit: rs.CommitIndex,
			}
			rs.Mu.Unlock()

			// 获取客户端
			rpcClient := client.GetLeaderClient(peerAddr)
			if rpcClient == nil {
				log.Log.CtxWarnf(ctx, "Leader心跳：获取节点[%d] %s 的客户端失败", idx, peerAddr)
				return
			}

			// 发送心跳
			timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			log.Log.CtxInfof(ctx, "Leader心跳：Leader %d 向节点[%d] %s 发送心跳", rs.NodeId, idx, peerAddr)

			resp, err := rpcClient.AppendEntries(timeoutCtx, heartbeatReq)
			if err != nil {
				log.Log.CtxWarnf(ctx, "Leader心跳：向节点[%d] %s 发送心跳失败: %v", idx, peerAddr, err)
				return
			}

			// 处理响应
			rs.Mu.Lock()
			defer rs.Mu.Unlock()

			if resp.Term > rs.CurrentTerm {
				log.Log.CtxWarnf(ctx, "Leader心跳：收到更高任期 %d > %d，即将转为Follower",
					resp.Term, rs.CurrentTerm)
				rs.CurrentTerm = resp.Term
				rs.VotedFor = -1
				rs.IsMaster = false

				// 更新全局配置
				configs.SetIsMaster(false)
			} else if resp.Success {
				mu.Lock()
				successCount++
				mu.Unlock()
				log.Log.CtxInfof(ctx, "Leader心跳：节点[%d] %s 确认心跳", idx, peerAddr)
			} else {
				log.Log.CtxWarnf(ctx, "Leader心跳：节点[%d] %s 拒绝心跳：term=%d",
					idx, peerAddr, resp.Term)
			}
		}(i, peer)
	}

	// 等待所有心跳完成或超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Log.CtxInfof(ctx, "Leader %d 完成心跳发送，成功数量: %d/%d",
			rs.NodeId, successCount, len(rs.Peers))
	case <-time.After(1500 * time.Millisecond):
		log.Log.CtxWarnf(ctx, "Leader %d 心跳发送超时，部分节点可能未响应", rs.NodeId)
	}
}

// ExpiredTimer 心跳超时检测
func (rs *RaftState) ExpiredTimer(ctx context.Context, masterAddr string, closeCh chan int) {
	log.Log.CtxInfof(ctx, "节点 %d 启动心跳检测，监控Leader %s", rs.NodeId, masterAddr)

	serverClient := client.GetLeaderClient(masterAddr)
	baseTimeout := rs.ElectionTimeout / 1000 // 转换为秒

	for {
		select {
		case <-closeCh:
			log.Log.CtxInfof(ctx, "节点 %d 停止心跳检测", rs.NodeId)
			return
		case <-time.After(time.Second * time.Duration(baseTimeout+rand.Intn(baseTimeout))): // 添加随机性
			// 检查是否已经成为Leader
			if rs.IsMaster {
				log.Log.CtxInfof(ctx, "节点 %d 已成为Leader，停止心跳检测", rs.NodeId)
				return
			}

			log.Log.CtxInfof(ctx, "节点 %d 心跳超时检测：向Leader %s 发送心跳检测", rs.NodeId, masterAddr)

			// 确保有效的客户端
			if serverClient == nil {
				log.Log.CtxWarnf(ctx, "节点 %d 心跳检测客户端为nil，创建新客户端", rs.NodeId)
				serverClient = client.GetLeaderClient(masterAddr)
				if serverClient == nil {
					log.Log.CtxErrorf(ctx, "节点 %d 无法创建心跳检测客户端，跳过本次检测", rs.NodeId)
					continue
				}
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*3) // 增加超时时间
			resp, err := serverClient.HeartBeat(timeoutCtx, &sever.HeartbeatReq{})
			cancel()

			if err != nil {
				log.Log.CtxWarnf(ctx, "节点 %d 心跳检测失败：无法连接到Leader %s，错误：%v",
					rs.NodeId, masterAddr, err)
				log.Log.CtxInfof(ctx, "节点 %d 心跳超时，准备开始新一轮选举", rs.NodeId)

				// 开始选举，但不退出循环
				rs.BeginVote()

				// 重新获取正确的Leader地址
				newConfig := configs.GetConfig()
				if newConfig.MasterAddr != masterAddr && newConfig.MasterAddr != "" {
					log.Log.CtxInfof(ctx, "节点 %d 发现新的Leader地址: %s", rs.NodeId, newConfig.MasterAddr)
					masterAddr = newConfig.MasterAddr
					serverClient = client.GetLeaderClient(masterAddr)
				}

				// 继续循环，不退出
				continue
			}

			log.Log.CtxInfof(ctx, "节点 %d 收到Leader心跳响应，更新集群节点列表", rs.NodeId)
			rs.SetPeers(resp.GetPeers())

			// 向状态机发送HeartbeatEvent事件，重置选举超时计时器
			stateMachine := GetRaftStateMachine()
			if stateMachine != nil {
				stateMachine.mu.Lock()
				stateMachine.lastHeartbeat = time.Now() // 直接更新lastHeartbeat
				stateMachine.mu.Unlock()
				stateMachine.eventCh <- RaftEvent{Type: HeartbeatEvent}
				log.Log.CtxInfof(ctx, "节点 %d 发送HeartbeatEvent事件到状态机，重置选举超时计时器", rs.NodeId)
			} else {
				log.Log.CtxErrorf(ctx, "节点 %d 获取状态机失败，无法发送心跳事件", rs.NodeId)
			}
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
