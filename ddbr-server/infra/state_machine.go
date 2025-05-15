package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/log"
)

// StateMachine 表示一个简单的键值存储状态机
type StateMachine struct {
	mu               sync.RWMutex
	data             map[string]string
	lastAppliedIndex int64
}

// 全局状态机实例
var (
	stateMachine     *StateMachine
	stateMachineOnce sync.Once
)

// CommandType 表示命令类型
type CommandType string

const (
	CommandTypeSet    CommandType = "set"
	CommandTypeGet    CommandType = "get"
	CommandTypeDelete CommandType = "delete"
)

// Command 表示状态机命令
type Command struct {
	Type  CommandType `json:"type"`
	Key   string      `json:"key"`
	Value string      `json:"value,omitempty"`
}

// GetStateMachine 获取状态机单例
func GetStateMachine() *StateMachine {
	stateMachineOnce.Do(func() {
		stateMachine = &StateMachine{
			data:             make(map[string]string),
			lastAppliedIndex: 0,
		}
	})
	return stateMachine
}

// Set 设置键值对
func (sm *StateMachine) Set(key, value string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.data[key] = value
}

// Get 获取键对应的值
func (sm *StateMachine) Get(key string) (string, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	value, exists := sm.data[key]
	return value, exists
}

// Delete 删除键值对
func (sm *StateMachine) Delete(key string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.data, key)
}

// GetLastAppliedIndex 获取最后应用的日志索引
func (sm *StateMachine) GetLastAppliedIndex() int64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastAppliedIndex
}

// SetLastAppliedIndex 设置最后应用的日志索引
func (sm *StateMachine) SetLastAppliedIndex(index int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastAppliedIndex = index
}

// GetAllData 获取所有数据（用于调试或快照）
func (sm *StateMachine) GetAllData() map[string]string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// 创建数据副本
	result := make(map[string]string, len(sm.data))
	for k, v := range sm.data {
		result[k] = v
	}
	return result
}

// ApplyLogToStateMachine 将日志条目应用到状态机
// 实现简单的命令执行: set:key:value, get:key, delete:key
func ApplyLogToStateMachine(entry sever.LogEntry) (string, error) {
	// 检查索引，防止重复应用
	sm := GetStateMachine()
	if entry.Index <= sm.GetLastAppliedIndex() {
		return "", fmt.Errorf("log entry already applied: %d", entry.Index)
	}

	// 解析命令
	var cmd Command

	// 支持两种格式：JSON格式和简单字符串格式
	if strings.HasPrefix(entry.Command, "{") {
		// JSON格式
		if err := json.Unmarshal([]byte(entry.Command), &cmd); err != nil {
			return "", fmt.Errorf("failed to parse command: %v", err)
		}
	} else {
		// 简单字符串格式: "set:key:value" 或 "get:key" 或 "delete:key"
		parts := strings.Split(entry.Command, ":")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid command format: %s", entry.Command)
		}

		cmdType := parts[0]
		key := parts[1]

		switch cmdType {
		case string(CommandTypeSet):
			if len(parts) != 3 {
				return "", fmt.Errorf("invalid set command format: %s", entry.Command)
			}
			cmd = Command{Type: CommandType(cmdType), Key: key, Value: parts[2]}
		case string(CommandTypeGet), string(CommandTypeDelete):
			cmd = Command{Type: CommandType(cmdType), Key: key}
		default:
			return "", fmt.Errorf("unknown command type: %s", cmdType)
		}
	}

	// 执行命令
	var result string
	ctx := context.Background()
	switch cmd.Type {
	case CommandTypeSet:
		sm.Set(cmd.Key, cmd.Value)
		result = fmt.Sprintf("OK")
		log.Log.CtxInfof(ctx, "StateMachine SET: %s = %s", cmd.Key, cmd.Value)
	case CommandTypeGet:
		value, exists := sm.Get(cmd.Key)
		if !exists {
			result = "(nil)"
		} else {
			result = value
		}
		log.Log.CtxInfof(ctx, "StateMachine GET: %s = %s", cmd.Key, result)
	case CommandTypeDelete:
		sm.Delete(cmd.Key)
		result = "OK"
		log.Log.CtxInfof(ctx, "StateMachine DELETE: %s", cmd.Key)
	default:
		return "", fmt.Errorf("unknown command type: %s", cmd.Type)
	}

	// 更新最后应用的日志索引
	sm.SetLastAppliedIndex(entry.Index)
	return result, nil
}
