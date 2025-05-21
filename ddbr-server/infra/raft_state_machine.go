package infra

import (
	"context"
	"encoding/json"
	"math/rand"
	"strings"
	"sync"
	"time"
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/configs"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/common"
	gatewayapi "zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/gateway"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"

	"github.com/cloudwego/kitex/client/callopt"
)

// 节点状态枚举
type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

func (s NodeState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return "Unknown"
	}
}

// 事件类型枚举
type EventType int

const (
	ElectionTimeout     EventType = iota // 选举超时
	HeartbeatTimeout                     // 心跳超时
	RequestVoteEvent                     // 收到投票请求
	RequestVoteResponse                  // 收到投票响应
	HeartbeatEvent                       // 收到心跳
	AppendEntriesEvent                   // 收到追加日志请求
	BecomeLeaderEvent                    // 成为Leader
	ShutdownEvent                        // 关闭事件
)

// 事件结构体
type RaftEvent struct {
	Type         EventType                   // 事件类型
	Term         int64                       // 任期号
	NodeID       int64                       // 节点ID
	VoteGranted  bool                        // 投票是否授予
	Source       string                      // 源节点地址
	VoteRequest  *sever.RequestVoteReq       // 投票请求
	VoteResponse chan *sever.RequestVoteResp // 投票响应通道
	Context      context.Context             // 上下文
}

// RaftStateMachine 基于状态机的Raft状态管理
type RaftStateMachine struct {
	// 节点标识与配置
	NodeID          int64
	ClusterID       int64
	Peers           []string
	ElectionTimeout time.Duration
	HeartbeatPeriod time.Duration

	// 状态机状态
	state       NodeState
	currentTerm int64
	votedFor    int64
	logs        []*sever.LogEntry
	commitIndex int64
	lastApplied int64

	// 仅Leader使用的字段
	nextIndex  map[string]int64
	matchIndex map[string]int64

	// 选举相关
	votesReceived int
	totalVotes    int

	// 心跳跟踪
	lastHeartbeat time.Time

	// 状态机通道
	eventCh    chan RaftEvent
	shutdownCh chan struct{}

	// 防止并发操作的锁
	mu sync.RWMutex

	// 服务器配置
	port        string
	gatewayHost string
	masterAddr  string
	isMaster    bool
}

// 全局状态机实例
var (
	globalStateMachine *RaftStateMachine
	smOnce             sync.Once
)

// GetRaftStateMachine 获取或创建状态机实例
func GetRaftStateMachine() *RaftStateMachine {
	smOnce.Do(func() {
		config := configs.GetConfig()

		globalStateMachine = &RaftStateMachine{
			state:           Follower,
			currentTerm:     0,
			votedFor:        -1,
			logs:            make([]*sever.LogEntry, 0),
			commitIndex:     0,
			lastApplied:     0,
			nextIndex:       make(map[string]int64),
			matchIndex:      make(map[string]int64),
			eventCh:         make(chan RaftEvent, 100), // 缓冲通道
			shutdownCh:      make(chan struct{}),
			lastHeartbeat:   time.Now(), // 初始化心跳时间
			ElectionTimeout: time.Duration(config.ElectionTimeout) * time.Millisecond,
			HeartbeatPeriod: time.Duration(config.HeartbeatPeriod) * time.Millisecond,
			NodeID:          *config.NodeID,
			ClusterID:       config.ClusterID,
			Peers:           config.Peers,
			port:            config.Port,
			gatewayHost:     config.GatewayHost,
			masterAddr:      config.MasterAddr,
			isMaster:        config.IsMaster,
		}

		// 启动状态机
		go globalStateMachine.run()
	})
	return globalStateMachine
}

// 启动状态机的主循环
func (sm *RaftStateMachine) run() {
	// 初始化选举计时器
	electionTimer := time.NewTimer(randomElectionTimeout(sm.ElectionTimeout))

	for {
		select {
		case <-sm.shutdownCh:
			return

		case <-electionTimer.C:
			// 选举超时，触发选举
			if sm.state != Leader {
				// 检查距离上次心跳的时间是否超过了选举超时的一半
				sm.mu.Lock()
				timeSinceLastHeartbeat := time.Since(sm.lastHeartbeat)
				halfTimeout := sm.ElectionTimeout / 2
				sm.mu.Unlock()

				// 只有当距离上次心跳时间超过选举超时时间的一半才触发选举
				if timeSinceLastHeartbeat > halfTimeout {
					log.Log.Infof("距离上次心跳已经过去 %v，超过阈值 %v，触发选举", timeSinceLastHeartbeat, halfTimeout)
					sm.handleElectionTimeout()
				} else {
					log.Log.Infof("忽略选举超时：距离上次心跳只有 %v，小于阈值 %v", timeSinceLastHeartbeat, halfTimeout)
				}
			}
			// 重置选举计时器
			electionTimer.Reset(randomElectionTimeout(sm.ElectionTimeout))

		case event := <-sm.eventCh:
			// 处理各类事件
			switch event.Type {
			case RequestVoteEvent:
				sm.handleRequestVote(event)
			case RequestVoteResponse:
				sm.handleRequestVoteResponse(event)
			case HeartbeatEvent:
				sm.handleHeartbeat(event)
				// 重置选举计时器
				if !electionTimer.Stop() {
					<-electionTimer.C
				}
				electionTimer.Reset(randomElectionTimeout(sm.ElectionTimeout))
			case BecomeLeaderEvent:
				sm.handleBecomeLeader()
			case ElectionTimeout:
				// 处理选举超时或手动触发选举
				sm.handleElectionTimeout()
				// 重置选举计时器
				if !electionTimer.Stop() {
					<-electionTimer.C
				}
				electionTimer.Reset(randomElectionTimeout(sm.ElectionTimeout))
			}
		}
	}
}

// UpdateConfig 用最新的配置更新状态机
func (sm *RaftStateMachine) UpdateConfig(config *configs.Config) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.Peers = config.Peers
	sm.ElectionTimeout = time.Duration(config.ElectionTimeout) * time.Millisecond
	sm.HeartbeatPeriod = time.Duration(config.HeartbeatPeriod) * time.Millisecond
	sm.masterAddr = config.MasterAddr
	sm.isMaster = config.IsMaster
}

// 处理投票请求
func (sm *RaftStateMachine) handleRequestVote(event RaftEvent) {
	req := event.VoteRequest
	resp := &sever.RequestVoteResp{
		Common: &common.Common{
			RespCode: 0,
			Message:  "success",
		},
	}

	ctx := event.Context
	sm.mu.RLock()

	log.Log.CtxInfof(ctx, "处理投票请求：候选人 %d，任期 %d，当前节点任期 %d，已投票给 %d",
		req.CandidateId, req.Term, sm.currentTerm, sm.votedFor)

	// 如果请求任期小于当前任期，拒绝投票
	if req.Term < sm.currentTerm {
		resp.Term = sm.currentTerm
		resp.VoteGranted = false
		resp.Common.Message = "term is smaller than current term"
		log.Log.CtxInfof(ctx, "拒绝投票：候选人任期较小")
		sm.mu.RUnlock()
		event.VoteResponse <- resp
		return
	}

	// 是否需要更新当前任期
	voteGranted := false
	currentTerm := sm.currentTerm

	// 如果请求任期大于当前任期，更新任期并重置投票状态
	if req.Term > sm.currentTerm {
		log.Log.CtxInfof(ctx, "更新任期：%d -> %d", sm.currentTerm, req.Term)
		sm.mu.RUnlock()
		sm.mu.Lock()
		sm.currentTerm = req.Term
		sm.votedFor = -1
		sm.state = Follower // 转为Follower
		currentTerm = sm.currentTerm
		sm.mu.Unlock()
		sm.mu.RLock()
	}

	// 记录投票结果
	resp.Term = currentTerm

	// 判断是否可以投票
	if sm.votedFor == -1 || sm.votedFor == req.CandidateId {
		// 检查日志完整性
		var lastLogTerm int64 = 0
		var lastLogIndex int64 = 0
		if len(sm.logs) > 0 {
			lastLog := sm.logs[len(sm.logs)-1]
			lastLogTerm = lastLog.Term
			lastLogIndex = lastLog.Index
		}

		// 候选人日志至少和我们一样新
		logOK := (req.LastLogTerm > lastLogTerm) ||
			(req.LastLogTerm == lastLogTerm && req.LastLogIndex >= lastLogIndex)

		if logOK {
			sm.mu.RUnlock()
			sm.mu.Lock()
			sm.votedFor = req.CandidateId
			voteGranted = true
			sm.mu.Unlock()
			// 这里需要重新获取读锁，因为后面还需要读取状态
			sm.mu.RLock()
			log.Log.CtxInfof(ctx, "授予投票：给候选人 %d", req.CandidateId)
		} else {
			resp.Common.Message = "candidate's log is not up-to-date"
			log.Log.CtxInfof(ctx, "拒绝投票：候选人日志不够新")
		}
	} else {
		resp.Common.Message = "already voted for another candidate"
		log.Log.CtxInfof(ctx, "拒绝投票：已投票给他人 %d", sm.votedFor)
	}

	resp.VoteGranted = voteGranted
	// 释放读锁
	sm.mu.RUnlock()
	event.VoteResponse <- resp
}

// 处理投票响应
func (sm *RaftStateMachine) handleRequestVoteResponse(event RaftEvent) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ctx := context.Background()

	// 如果不再是候选人或者任期已经变化，忽略响应
	if sm.state != Candidate || sm.currentTerm != event.Term {
		log.Log.CtxInfof(ctx, "忽略选票：状态已变更 (当前状态=%s, 当前任期=%d, 事件任期=%d)",
			sm.state, sm.currentTerm, event.Term)
		return
	}

	// 如果获得选票
	if event.VoteGranted {
		sm.votesReceived++
		log.Log.CtxInfof(ctx, "节点获得选票，当前得票数 %d/%d", sm.votesReceived, sm.totalVotes)

		// 检查是否获得了多数选票
		votesNeeded := sm.totalVotes/2 + 1
		if sm.votesReceived >= votesNeeded {
			log.Log.CtxInfof(ctx, "节点获得多数选票 (%d/%d)，将成为Leader",
				sm.votesReceived, sm.totalVotes)

			// 使用另一个事件触发成为Leader的处理
			sm.mu.Unlock()
			sm.eventCh <- RaftEvent{Type: BecomeLeaderEvent}
			sm.mu.Lock()
		}
	} else {
		log.Log.CtxInfof(ctx, "节点未获得选票，来自 %s", event.Source)
	}
}

// 处理心跳
func (sm *RaftStateMachine) handleHeartbeat(event RaftEvent) {
	// 重置到Follower状态
	sm.mu.Lock()
	sm.state = Follower
	sm.lastHeartbeat = time.Now() // 更新最后一次心跳时间
	log.Log.Infof("节点 %d 收到心跳，更新最后心跳时间为 %v", sm.NodeID, sm.lastHeartbeat)
	sm.mu.Unlock()

	// 具体心跳处理逻辑
}

// 处理选举超时
func (sm *RaftStateMachine) handleElectionTimeout() {
	sm.mu.Lock()

	// 只有Follower和Candidate才会进行选举
	if sm.state == Leader {
		sm.mu.Unlock()
		return
	}

	// 转变为候选人
	sm.state = Candidate
	sm.currentTerm++
	sm.votedFor = sm.NodeID // 先投自己一票
	currentTerm := sm.currentTerm

	// 重置选票计数
	sm.votesReceived = 1              // 包括自己的一票
	sm.totalVotes = len(sm.Peers) + 1 // 包括自己

	// 复制peers列表以便在锁外使用
	peers := make([]string, len(sm.Peers))
	copy(peers, sm.Peers)
	nodeId := sm.NodeID

	ctx := context.Background()
	log.Log.CtxInfof(ctx, "选举超时：节点 %d 成为候选人，任期 %d", sm.NodeID, sm.currentTerm)

	// 如果是单节点集群，直接成为Leader
	singleNode := len(peers) == 0 || sm.totalVotes <= 1

	// 准备日志信息
	var lastLogTerm int64 = 0
	var lastLogIndex int64 = 0
	if len(sm.logs) > 0 {
		lastLog := sm.logs[len(sm.logs)-1]
		lastLogTerm = lastLog.Term
		lastLogIndex = lastLog.Index
	}

	// 现在解锁，避免在发送RPC时长时间持有锁
	sm.mu.Unlock()

	// 单节点情况特殊处理
	if singleNode {
		log.Log.CtxInfof(ctx, "单节点集群：节点 %d 直接成为Leader", nodeId)
		// 发送成为Leader事件
		sm.eventCh <- RaftEvent{Type: BecomeLeaderEvent}
		return
	}

	// 准备投票请求
	voteReq := &sever.RequestVoteReq{
		Term:         currentTerm,
		CandidateId:  nodeId,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}

	// 向所有其他节点发送RequestVote RPC
	var wg sync.WaitGroup

	for i, peer := range peers {
		if peer == "" {
			continue
		}

		wg.Add(1)
		go func(idx int, peerAddr string) {
			defer wg.Done()

			log.Log.CtxInfof(ctx, "选举：向节点 %s 发送投票请求", peerAddr)

			// 获取RPC客户端
			rpcClient := client.GetLeaderClient(peerAddr)
			if rpcClient == nil {
				log.Log.CtxWarnf(ctx, "选举：获取节点 %s 的客户端失败", peerAddr)
				return
			}

			// 发送投票请求
			voteResp, err := rpcClient.RequestVote(
				ctx,
				voteReq,
				callopt.WithConnectTimeout(time.Second*1),
				callopt.WithRPCTimeout(time.Second*2))

			if err != nil {
				log.Log.CtxErrorf(ctx, "选举：向节点 %s 发送投票请求失败: %v", peerAddr, err)
				return
			}

			log.Log.CtxInfof(ctx, "选举：收到节点 %s 的投票响应: %+v", peerAddr, voteResp)

			// 将投票结果作为事件发送到状态机
			event := RaftEvent{
				Type:        RequestVoteResponse,
				Term:        currentTerm,
				NodeID:      nodeId,
				VoteGranted: voteResp.VoteGranted,
				Source:      peerAddr,
			}

			sm.eventCh <- event

		}(i, peer)
	}

	// 启动一个goroutine等待所有投票请求完成
	go func() {
		wg.Wait()
		log.Log.CtxInfof(ctx, "选举：所有投票请求已完成")
	}()
}

// 成为Leader
func (sm *RaftStateMachine) handleBecomeLeader() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.state != Candidate {
		return
	}

	// 检查Peers列表，如果为空，记录警告
	if len(sm.Peers) == 0 {
		log.Log.Infof("警告：节点 %d 成为Leader前Peers列表为空!", sm.NodeID)
	}

	// 在修改状态前记录当前Peers列表
	log.Log.Infof("节点 %d 成为Leader前的Peers列表: %v", sm.NodeID, sm.Peers)

	sm.state = Leader
	sm.isMaster = true

	// 更新全局配置
	configs.SetIsMaster(true)

	ctx := context.Background()
	log.Log.CtxInfof(ctx, "节点 %d 成为Leader，任期 %d，集群节点数 %d",
		sm.NodeID, sm.currentTerm, len(sm.Peers))

	// 初始化Leader状态
	for _, peer := range sm.Peers {
		lastLogIndex := int64(0)
		if len(sm.logs) > 0 {
			lastLogIndex = sm.logs[len(sm.logs)-1].Index
		}
		sm.nextIndex[peer] = lastLogIndex + 1
		sm.matchIndex[peer] = 0
	}

	// 尝试向Gateway注册成为Leader并获取最新节点列表
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Log.Errorf("Leader注册过程发生panic: %v", r)
			}
		}()

		// 尝试获取最新的节点列表
		sm.updateClusterInfo()

		// 启动心跳
		sm.startHeartbeats()
	}()
}

// updateClusterInfo 尝试更新集群节点信息
func (sm *RaftStateMachine) updateClusterInfo() {
	ctx := context.Background()
	// 尝试向网关注册成为Leader
	gatewayClient := client.GetGatewayClient()
	if gatewayClient == nil {
		log.Log.CtxWarnf(ctx, "无法获取Gateway客户端，使用现有节点列表")
		return
	}

	// 发送SetLeader请求
	_, err := gatewayClient.SetLeader(ctx, &gatewayapi.SetLeaderReq{})
	if err != nil {
		log.Log.CtxWarnf(ctx, "向Gateway注册Leader失败: %v", err)
		return
	}

	// 获取当前注册的节点列表
	registerResp, err := gatewayClient.RegisterSever(ctx, &gatewayapi.RegisterSeverReq{
		ServerHost: sm.port,
		NodeId:     sm.NodeID,
		IsNew:      false,
	})

	if err != nil {
		log.Log.CtxWarnf(ctx, "获取最新节点列表失败: %v", err)
		return
	}

	// 更新节点列表
	sm.mu.Lock()
	oldPeers := sm.Peers
	if registerResp != nil && len(registerResp.SeverHostSever) > 0 {
		// 过滤掉自己的地址
		newPeers := make([]string, 0, len(registerResp.SeverHostSever))
		for _, host := range registerResp.SeverHostSever {
			// 如果包含端口，直接使用
			if strings.Contains(host, ":") {
				newPeers = append(newPeers, host)
			} else {
				// 否则加上默认端口
				newPeers = append(newPeers, host+":8080")
			}
		}

		sm.Peers = newPeers
		log.Log.CtxInfof(ctx, "Leader更新节点列表: %v -> %v", oldPeers, newPeers)
	} else {
		log.Log.CtxWarnf(ctx, "Gateway返回空节点列表，保持原有列表: %v", oldPeers)
	}
	sm.mu.Unlock()
}

// 启动心跳发送
func (sm *RaftStateMachine) startHeartbeats() {
	ticker := time.NewTicker(sm.HeartbeatPeriod)
	defer ticker.Stop()

	ctx := context.Background()

	// 立即发送第一次心跳
	sm.sendHeartbeats(ctx)

	for {
		sm.mu.RLock()
		if sm.state != Leader {
			sm.mu.RUnlock()
			return
		}
		sm.mu.RUnlock()

		select {
		case <-ticker.C:
			// 发送心跳
			sm.sendHeartbeats(ctx)
		case <-sm.shutdownCh:
			return
		}
	}
}

// 发送心跳
func (sm *RaftStateMachine) sendHeartbeats(ctx context.Context) {
	sm.mu.RLock()
	peers := make([]string, len(sm.Peers))
	copy(peers, sm.Peers)
	currentTerm := sm.currentTerm
	nodeID := sm.NodeID
	commitIndex := sm.commitIndex
	sm.mu.RUnlock()

	log.Log.CtxInfof(ctx, "Leader %d 开始发送心跳，当前集群节点数: %d", nodeID, len(peers))

	if len(peers) == 0 {
		log.Log.CtxInfof(ctx, "Leader %d 没有其他节点，跳过心跳发送", nodeID)
		return
	}

	// 向每个Follower发送心跳
	var wg sync.WaitGroup

	for i, peer := range peers {
		if peer == "" {
			continue
		}

		wg.Add(1)
		go func(idx int, peerAddr string) {
			defer wg.Done()

			// 获取RPC客户端
			rpcClient := client.GetLeaderClient(peerAddr)
			if rpcClient == nil {
				log.Log.CtxWarnf(ctx, "心跳：获取节点 %s 的客户端失败", peerAddr)
				return
			}

			// 准备心跳请求
			heartbeatReq := &sever.AppendEntriesReq{
				Term:         currentTerm,
				LeaderId:     nodeID,
				PrevLogIndex: 0,
				PrevLogTerm:  0,
				Entries:      []string{}, // 空日志表示心跳
				LeaderCommit: commitIndex,
			}

			// 发送心跳
			timeoutCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			resp, err := rpcClient.AppendEntries(timeoutCtx, heartbeatReq)
			if err != nil {
				log.Log.CtxWarnf(ctx, "心跳：向节点 %s 发送心跳失败: %v", peerAddr, err)
				return
			}

			// 处理响应
			if resp.Term > currentTerm {
				// 发现更高任期，切换到Follower状态
				sm.mu.Lock()
				if resp.Term > sm.currentTerm {
					log.Log.CtxWarnf(ctx, "心跳：发现更高任期 %d > %d，转为Follower",
						resp.Term, sm.currentTerm)
					sm.currentTerm = resp.Term
					sm.votedFor = -1
					sm.state = Follower
					sm.isMaster = false

					// 更新全局配置
					configs.SetIsMaster(false)
				}
				sm.mu.Unlock()
			}

		}(i, peer)
	}

	// 等待所有心跳完成
	wg.Wait()
}

// RequestVote 处理投票请求的API接口
func (sm *RaftStateMachine) RequestVote(ctx context.Context, req *sever.RequestVoteReq) (*sever.RequestVoteResp, error) {
	respCh := make(chan *sever.RequestVoteResp, 1)

	// 创建投票请求事件
	event := RaftEvent{
		Type:         RequestVoteEvent,
		VoteRequest:  req,
		VoteResponse: respCh,
		Context:      ctx,
	}

	// 发送事件到状态机
	sm.eventCh <- event

	// 等待处理结果
	select {
	case resp := <-respCh:
		return resp, nil
	case <-time.After(500 * time.Millisecond):
		// 超时处理
		return &sever.RequestVoteResp{
			Common: &common.Common{
				RespCode: 1,
				Message:  "request processing timeout",
			},
			VoteGranted: false,
		}, nil
	}
}

// BeginVote 开始选举
func (sm *RaftStateMachine) BeginVote() {
	ctx := context.Background()

	// 检查距离上次心跳的时间
	sm.mu.RLock()
	timeSinceLastHeartbeat := time.Since(sm.lastHeartbeat)
	halfTimeout := sm.ElectionTimeout / 2
	isLeader := sm.state == Leader
	nodeID := sm.NodeID
	sm.mu.RUnlock()

	// 如果是Leader或者距离上次心跳时间很短，则跳过选举
	if isLeader {
		log.Log.CtxInfof(ctx, "节点 %d 已是Leader，跳过选举", nodeID)
		return
	}

	// 只有当距离上次心跳时间超过选举超时的一半才允许开始选举
	if timeSinceLastHeartbeat <= halfTimeout {
		log.Log.CtxInfof(ctx, "节点 %d 跳过选举：距离上次心跳仅 %v，小于阈值 %v",
			nodeID, timeSinceLastHeartbeat, halfTimeout)
		return
	}

	log.Log.CtxInfof(ctx, "节点 %d 手动触发选举，距离上次心跳已 %v", nodeID, timeSinceLastHeartbeat)

	// 触发选举超时事件
	sm.eventCh <- RaftEvent{
		Type:    ElectionTimeout,
		Context: ctx,
	}
}

// randomElectionTimeout 返回随机化的选举超时时间
func randomElectionTimeout(base time.Duration) time.Duration {
	return base + time.Duration(rand.Int63n(int64(base)))
}

// AppendEntries 处理追加日志请求
func (sm *RaftStateMachine) AppendEntries(ctx context.Context, req *sever.AppendEntriesReq) (*sever.AppendEntriesResp, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	resp := &sever.AppendEntriesResp{
		Term:    sm.currentTerm,
		Success: false,
	}

	// 更新最后一次心跳时间
	sm.lastHeartbeat = time.Now()
	log.Log.CtxInfof(ctx, "节点 %d 收到来自Leader %d 的AppendEntries请求，更新心跳时间", sm.NodeID, req.LeaderId)

	// 心跳处理 - 重置选举计时器由run()中的事件处理完成
	sm.eventCh <- RaftEvent{Type: HeartbeatEvent}

	// 如果请求的任期小于当前任期，拒绝
	if req.Term < sm.currentTerm {
		log.Log.CtxInfof(ctx, "拒绝追加日志：任期 %d < 当前任期 %d", req.Term, sm.currentTerm)
		return resp, nil
	}

	// 如果是更高任期，更新当前任期并转为Follower
	if req.Term > sm.currentTerm {
		log.Log.CtxInfof(ctx, "发现更高任期 %d > %d，转为Follower", req.Term, sm.currentTerm)
		sm.currentTerm = req.Term
		sm.votedFor = -1
		sm.state = Follower
		sm.isMaster = false
		configs.SetIsMaster(false)
	}

	// 心跳请求 - 空日志条目
	if len(req.Entries) == 0 {
		// 心跳请求成功
		resp.Success = true
		return resp, nil
	}

	// 实际日志追加逻辑
	// 1. 一致性检查：验证PrevLogIndex和PrevLogTerm
	// 确保我们有足够的日志条目
	if req.PrevLogIndex > int64(len(sm.logs)) {
		log.Log.CtxInfof(ctx, "一致性检查失败：日志长度 %d < PrevLogIndex %d",
			len(sm.logs), req.PrevLogIndex)
		resp.Success = false
		return resp, nil
	}

	// 检查前一个日志条目的任期是否匹配
	if req.PrevLogIndex > 0 {
		// 由于日志索引从1开始，我们需要将索引-1获取切片位置
		prevLogPos := req.PrevLogIndex - 1

		// 如果PrevLogIndex处没有日志或任期不匹配，拒绝请求
		if prevLogPos >= int64(len(sm.logs)) || sm.logs[prevLogPos].Term != req.PrevLogTerm {
			log.Log.CtxInfof(ctx, "一致性检查失败：PrevLogIndex=%d 处的任期不匹配或不存在",
				req.PrevLogIndex)
			resp.Success = false
			return resp, nil
		}
	}

	// 2. 处理新日志条目
	// 将字符串日志条目转换为LogEntry对象
	newEntries := make([]*sever.LogEntry, 0, len(req.Entries))

	// 计算新日志条目的起始索引
	startIndex := req.PrevLogIndex + 1

	// 处理每个新的日志条目
	for i, entryStr := range req.Entries {
		// 反序列化日志条目
		logEntry, err := deserializeLogEntry(entryStr)
		if err != nil {
			log.Log.CtxErrorf(ctx, "反序列化日志条目失败：%v", err)
			// 如果失败，创建一个包含原始命令的简单日志条目
			logEntry = &sever.LogEntry{
				Term:    req.Term,
				Index:   startIndex + int64(i),
				Command: entryStr,
			}
		}

		// 确保索引正确
		logEntry.Index = startIndex + int64(i)
		logEntry.Term = req.Term

		newEntries = append(newEntries, logEntry)
	}

	// 3. 查找已有日志与新日志的冲突点
	conflictIndex := -1

	for i, newEntry := range newEntries {
		// 计算这个日志条目对应的日志数组索引
		logIndex := int(newEntry.Index - 1)

		// 如果索引超出现有日志长度，没有冲突
		if logIndex >= len(sm.logs) {
			break
		}

		// 如果任期不同，标记为冲突点
		if sm.logs[logIndex].Term != newEntry.Term {
			conflictIndex = i
			break
		}
	}

	// 4. 如果发现冲突，删除冲突点及之后的所有日志
	if conflictIndex != -1 {
		// 计算冲突的日志索引
		conflictLogIndex := newEntries[conflictIndex].Index

		// 截断日志，删除从冲突点开始的所有日志
		sm.logs = sm.logs[:conflictLogIndex-1]

		log.Log.CtxInfof(ctx, "检测到日志冲突，截断日志到索引 %d", conflictLogIndex-1)
	}

	// 5. 追加新日志条目
	// 计算需要追加的新日志条目
	appendStartIndex := 0
	if len(sm.logs) > 0 {
		// 计算从哪个位置开始追加
		appendStartIndex = len(sm.logs)
	}

	// 追加新日志
	if appendStartIndex < len(newEntries) {
		log.Log.CtxInfof(ctx, "追加 %d 条新日志", len(newEntries)-appendStartIndex)
		sm.logs = append(sm.logs, newEntries[appendStartIndex:]...)
	}

	// 6. 更新commitIndex
	if req.LeaderCommit > sm.commitIndex {
		// 计算新的commitIndex
		oldCommitIndex := sm.commitIndex
		sm.commitIndex = min(req.LeaderCommit, int64(len(sm.logs)))

		log.Log.CtxInfof(ctx, "更新commitIndex: %d -> %d", oldCommitIndex, sm.commitIndex)

		// 尝试应用新提交的日志
		go func() {
			if err := sm.applyCommittedLogs(); err != nil {
				log.Log.CtxErrorf(ctx, "应用已提交日志失败: %v", err)
			}
		}()
	}

	resp.Success = true
	return resp, nil
}

// min 返回两个整数中的较小值
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// deserializeLogEntry 将字符串反序列化为LogEntry
func deserializeLogEntry(data string) (*sever.LogEntry, error) {
	// 尝试使用JSON反序列化
	var entry sever.LogEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		// 如果无法反序列化，创建一个简单条目
		return &sever.LogEntry{
			Command: data,
		}, err
	}
	return &entry, nil
}

// applyCommittedLogs 应用已提交但未应用的日志到状态机
func (sm *RaftStateMachine) applyCommittedLogs() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 检查是否有新的已提交日志需要应用
	if sm.lastApplied >= sm.commitIndex {
		return nil
	}

	log.Log.Infof("应用日志：lastApplied=%d, commitIndex=%d", sm.lastApplied, sm.commitIndex)

	// 获取需要应用的日志条目
	for i := sm.lastApplied + 1; i <= sm.commitIndex && i-1 < int64(len(sm.logs)); i++ {
		// 将日志条目应用到状态机
		entry := sm.logs[i-1]

		// 跳过已经应用过的日志
		if entry.Index <= sm.lastApplied {
			continue
		}

		log.Log.Infof("应用日志条目 %d: %s", entry.Index, entry.Command)

		// 应用日志到状态机
		result, err := ApplyLogToStateMachine(*entry)
		if err != nil {
			if strings.Contains(err.Error(), "log entry already applied") {
				// 如果是重复应用的错误，记录并继续
				log.Log.Warnf("日志条目 %d 已应用，跳过: %v", entry.Index, err)
			} else {
				// 其他错误则返回
				log.Log.Errorf("应用日志条目 %d 失败: %v", entry.Index, err)
				return err
			}
		} else {
			log.Log.Infof("日志条目 %d 应用成功: %s", entry.Index, result)
		}

		// 更新lastApplied
		sm.lastApplied = entry.Index
	}

	return nil
}

// GetCurrentTerm 获取当前任期
func (sm *RaftStateMachine) GetCurrentTerm() int64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentTerm
}

// ExpiredTimer 心跳超时检测
func (sm *RaftStateMachine) ExpiredTimer(ctx context.Context, masterAddr string, closeCh chan int) {
	log.Log.CtxInfof(ctx, "节点 %d 启动心跳检测，监控Leader %s", sm.NodeID, masterAddr)

	// 获取客户端连接
	serverClient := client.GetLeaderClient(masterAddr)
	if serverClient == nil {
		log.Log.CtxWarnf(ctx, "无法获取Leader客户端，尝试重新创建")
		serverClient = client.GetLeaderClient(masterAddr)
		if serverClient == nil {
			log.Log.CtxErrorf(ctx, "无法创建Leader客户端，心跳检测可能无法正常工作")
		}
	}

	// 计算心跳间隔时间 (1/3 选举超时)
	heartbeatInterval := sm.ElectionTimeout / 3

	// 启动心跳检测循环
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-closeCh:
			log.Log.CtxInfof(ctx, "节点 %d 停止心跳检测", sm.NodeID)
			return

		case <-ticker.C:
			// 检查是否已经成为Leader
			sm.mu.RLock()
			isLeader := sm.state == Leader
			nodeID := sm.NodeID
			sm.mu.RUnlock()

			if isLeader {
				log.Log.CtxInfof(ctx, "节点 %d 已成为Leader，停止心跳检测", nodeID)
				return
			}

			// 检查客户端是否有效
			if serverClient == nil {
				log.Log.CtxWarnf(ctx, "Leader客户端为nil，尝试重新创建")
				serverClient = client.GetLeaderClient(masterAddr)
				if serverClient == nil {
					log.Log.CtxErrorf(ctx, "创建Leader客户端失败，跳过本次心跳检测")
					continue
				}
			}

			log.Log.CtxInfof(ctx, "节点 %d 向Leader %s 发送心跳检测", nodeID, masterAddr)

			// 设置超时上下文
			timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*3)

			// 主动向Leader发送心跳检测
			resp, err := serverClient.HeartBeat(timeoutCtx, &sever.HeartbeatReq{})
			cancel()

			if err != nil {
				log.Log.CtxWarnf(ctx, "心跳检测失败：无法连接到Leader %s，错误：%v",
					masterAddr, err)

				// 检查距离上次心跳的时间
				sm.mu.RLock()
				timeSinceLastHeartbeat := time.Since(sm.lastHeartbeat)
				electionTimeout := sm.ElectionTimeout
				sm.mu.RUnlock()

				// 如果已经超过选举超时时间，触发选举
				if timeSinceLastHeartbeat > electionTimeout {
					log.Log.CtxInfof(ctx, "心跳检测失败且超过选举超时时间 %v，准备开始新一轮选举",
						electionTimeout)

					// 手动触发选举
					sm.BeginVote()

					// 尝试获取新的Leader地址
					newConfig := configs.GetConfig()
					if newConfig.MasterAddr != masterAddr && newConfig.MasterAddr != "" {
						log.Log.CtxInfof(ctx, "发现新的Leader地址: %s", newConfig.MasterAddr)
						masterAddr = newConfig.MasterAddr
						serverClient = client.GetLeaderClient(masterAddr)
					}
				} else {
					log.Log.CtxInfof(ctx, "心跳检测失败但未超过选举超时时间，继续等待")
				}

				continue
			}

			// 处理心跳响应成功的情况
			log.Log.CtxInfof(ctx, "收到Leader %s 心跳响应，准备更新节点状态", masterAddr)

			// 更新当前时间作为最后心跳接收时间
			currentTime := time.Now()

			// 更新节点列表
			peers := resp.GetPeers()

			// 更新本地状态
			sm.mu.Lock()
			sm.lastHeartbeat = currentTime
			if len(peers) > 0 {
				oldPeers := sm.Peers
				sm.Peers = peers
				log.Log.CtxInfof(ctx, "更新节点列表: %v -> %v", oldPeers, peers)
			}
			sm.mu.Unlock()

			// 发送心跳事件以重置选举计时器
			sm.eventCh <- RaftEvent{
				Type:    HeartbeatEvent,
				Context: ctx,
			}

			log.Log.CtxInfof(ctx, "心跳检测完成，已重置选举计时器")
		}
	}
}
