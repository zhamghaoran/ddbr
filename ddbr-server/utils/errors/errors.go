package errors

import (
	"fmt"
	"zhamghaoran/ddbr-server/log"
)

// ErrorType 错误类型枚举
type ErrorType string

const (
	// 错误类型常量
	ErrorTypeInternal    ErrorType = "Internal"    // 内部错误
	ErrorTypeValidation  ErrorType = "Validation"  // 验证错误
	ErrorTypeConnection  ErrorType = "Connection"  // 连接错误
	ErrorTypePermission  ErrorType = "Permission"  // 权限错误
	ErrorTypeRaft        ErrorType = "Raft"        // Raft相关错误
	ErrorTypeConfig      ErrorType = "Config"      // 配置错误
	ErrorTypeUnavailable ErrorType = "Unavailable" // 服务不可用
)

// AppError 应用错误结构体
type AppError struct {
	Type    ErrorType // 错误类型
	Code    int       // 错误码
	Message string    // 错误信息
	Err     error     // 原始错误
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap 返回原始错误
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError 创建新的应用错误
func NewAppError(errType ErrorType, code int, message string, err error) *AppError {
	return &AppError{
		Type:    errType,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// InternalError 创建内部错误
func InternalError(message string, err error) *AppError {
	return NewAppError(ErrorTypeInternal, 500, message, err)
}

// ValidationError 创建验证错误
func ValidationError(message string, err error) *AppError {
	return NewAppError(ErrorTypeValidation, 400, message, err)
}

// ConnectionError 创建连接错误
func ConnectionError(message string, err error) *AppError {
	return NewAppError(ErrorTypeConnection, 503, message, err)
}

// RaftError 创建Raft相关错误
func RaftError(message string, err error) *AppError {
	return NewAppError(ErrorTypeRaft, 500, message, err)
}

// ConfigError 创建配置错误
func ConfigError(message string, err error) *AppError {
	return NewAppError(ErrorTypeConfig, 500, message, err)
}

// HandleError 处理错误并记录日志
func HandleError(err error, recoverable bool) {
	if err == nil {
		return
	}

	// 记录错误
	log.Log.Errorf("Error occurred: %v", err)

	// 如果不可恢复，触发panic
	if !recoverable {
		panic(err)
	}
}

// WrapError 包装错误
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
