package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"zhamghaoran/ddbr-server/log"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/klog"
)

// LogMiddleware 创建一个日志中间件，记录RPC请求和响应
func LogMiddleware() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) (err error) {
			startTime := time.Now()
			// 记录请求信息
			reqJSON, _ := json.MarshalIndent(req, "", "  ")
			log.Log.CtxInfof(ctx, "RPC调用开始 - 请求内容:\n%s", string(reqJSON))

			// 调用下一个处理器
			err = next(ctx, req, resp)

			// 计算耗时
			duration := time.Since(startTime)

			// 记录响应信息
			respJSON, _ := json.MarshalIndent(resp, "", "  ")

			// 生成日志信息
			logMsg := fmt.Sprintf("RPC调用完成 - 耗时: %v\n请求:\n%s\n响应:\n%s",
				duration, string(reqJSON), string(respJSON))

			if err != nil {
				// 如果有错误，记录错误信息
				log.Log.CtxErrorf(ctx, "%s\n错误: %v", logMsg, err)
			} else {
				// 正常记录
				log.Log.CtxInfof(ctx, "%s", logMsg)
			}

			// 以原始的错误返回
			return err
		}
	}
}

// SimplifiedLogMiddleware 简化版日志中间件，避免大量日志输出
func SimplifiedLogMiddleware() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) (err error) {
			startTime := time.Now()

			// 获取方法名
			methodName := ctx.Value("kitex-call-func")
			if methodName == nil {
				methodName = "Unknown"
			}

			// 简单记录请求开始
			klog.Infof("RPC调用开始: %v", methodName)

			// 调用下一个处理器
			err = next(ctx, req, resp)

			// 计算耗时
			duration := time.Since(startTime)

			// 简单记录请求结束状态
			if err != nil {
				klog.Errorf("RPC调用失败: %v, 耗时: %v, 错误: %v", methodName, duration, err)
			} else {
				klog.Infof("RPC调用成功: %v, 耗时: %v", methodName, duration)
			}

			return err
		}
	}
}
