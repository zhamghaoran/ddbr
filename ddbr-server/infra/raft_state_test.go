package infra

import (
	"testing"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
)

func TestRaftStateIntegration(t *testing.T) {
	// 获取RaftState实例
	raftState := GetRaftState()
	if raftState == nil {
		t.Fatal("Failed to get RaftState instance")
	}

	// 测试配置相关方法
	config := RaftConfig{
		NodeId:          new(int64),
		ClusterId:       123,
		Peers:           []string{"peer1", "peer2"},
		ElectionTimeout: 2000,
		HeartbeatPeriod: 200,
		DataDir:         "test_data",
		SnapshotCount:   5000,
		Port:            "8080",
		IsMaster:        true,
	}
	*config.NodeId = 456

	// 更新配置
	raftState.UpdateConfig(config)

	// 验证配置更新是否生效
	retrievedConfig := raftState.GetConfig()
	if retrievedConfig.ClusterId != config.ClusterId {
		t.Errorf("Config ClusterId not updated correctly. Expected %d, got %d",
			config.ClusterId, retrievedConfig.ClusterId)
	}
	if retrievedConfig.ElectionTimeout != config.ElectionTimeout {
		t.Errorf("Config ElectionTimeout not updated correctly. Expected %d, got %d",
			config.ElectionTimeout, retrievedConfig.ElectionTimeout)
	}
	if len(retrievedConfig.Peers) != len(config.Peers) {
		t.Errorf("Config Peers not updated correctly. Expected %v, got %v",
			config.Peers, retrievedConfig.Peers)
	}

	// 测试状态相关方法
	raftState.SetCurrentTerm(10)
	if term := raftState.GetCurrentTerm(); term != 10 {
		t.Errorf("CurrentTerm not set correctly. Expected 10, got %d", term)
	}

	raftState.SetVotedFor(789)
	if votedFor := raftState.GetVotedFor(); votedFor != 789 {
		t.Errorf("VotedFor not set correctly. Expected 789, got %d", votedFor)
	}

	// 测试日志相关方法
	testLogs := []*sever.LogEntry{
		{Term: 1, Index: 1, Command: "cmd1"},
		{Term: 2, Index: 2, Command: "cmd2"},
	}
	raftState.SetLogs(testLogs)

	retrievedLogs := raftState.GetLogs()
	if len(retrievedLogs) != len(testLogs) {
		t.Errorf("Logs not set correctly. Expected length %d, got %d",
			len(testLogs), len(retrievedLogs))
	}

	// 测试节点ID方法
	raftState.SetNodeId(1234)
	if nodeId := raftState.GetNodeId(); nodeId != 1234 {
		t.Errorf("NodeId not set correctly. Expected 1234, got %d", nodeId)
	}

	// 测试Peers方法
	newPeers := []string{"peer3", "peer4", "peer5"}
	raftState.SetPeers(newPeers)
	retrievedConfig = raftState.GetConfig()
	if len(retrievedConfig.Peers) != len(newPeers) {
		t.Errorf("Peers not set correctly. Expected length %d, got %d",
			len(newPeers), len(retrievedConfig.Peers))
	}
}
