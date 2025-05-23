package main

import (
	"context"
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/server"

	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/configs"
	"zhamghaoran/ddbr-server/infra"
	ddbr "zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever/server"
	"zhamghaoran/ddbr-server/log"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "configs/server_config.json", "配置文件路径")
	master := flag.Bool("master", false, "是否为master节点")
	flag.Parse()
	log.Log.Infof("当前节点状态:%v", *master)
	// 检查配置文件是否存在
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Log.Fatalf("配置文件不存在: %s", *configPath)
	}

	log.Log.Infof("启动节点，配置路径: %s, 是否为master: %v", *configPath, *master)

	// 初始化资源（会调用configs.LoadConfig加载配置）
	if err := infra.InitializeResources(*configPath, *master); err != nil {
		log.Log.Fatalf("初始化资源失败: %v", err)
	}

	// 获取最终状态
	config := configs.GetConfig()

	if config.IsMaster {
		log.Log.Infof("节点成功启动为 LEADER 节点")
	} else {
		log.Log.Infof("节点成功启动为 FOLLOWER 节点")
	}

	// 设置优雅关闭
	setupGracefulShutdown()
	// 启动服务器
	startServer()
}

// startServer 启动RPC服务器
func startServer() {
	// 获取配置
	config := configs.GetConfig()
	port := config.Port

	// 如果是Follower节点，启动心跳检测
	if !config.IsMaster {
		log.Log.Infof("Follower节点启动心跳检测，监控Leader: %s", config.MasterAddr)

		// 使用新状态机替代旧的RaftState
		stateMachine := infra.GetRaftStateMachine()

		// 创建关闭通道
		closeCh := make(chan int)

		// 启动心跳检测
		go stateMachine.ExpiredTimer(context.Background(), config.MasterAddr, closeCh)

		// 将关闭通道保存，以便在服务关闭时使用
		// 可以通过全局变量或其他方式保存
	}

	addr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:"+port)
	code := thrift.NewThriftCodecWithConfig(thrift.FrugalRead | thrift.FrugalWrite)

	// 创建服务实例，添加日志中间件
	svr := ddbr.NewServer(
		new(ServerImpl),
		server.WithPayloadCodec(code),
		server.WithServiceAddr(addr),
		//server.WithMiddleware(middleware.LogMiddleware()), // 添加详细日志中间件
		// 如果日志太多，可以使用简化版中间件：server.WithMiddleware(middleware.SimplifiedLogMiddleware()),
	)

	log.Log.Infof("服务器正在启动，监听端口: %s", port)

	// 启动服务
	if err := svr.Run(); err != nil {
		log.Log.Fatalf("启动服务器失败: %v", err)
	}
}

// setupGracefulShutdown 设置优雅关闭
func setupGracefulShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Log.Infof("接收到信号 %v, 准备关闭服务...", sig)

		// 执行清理操作
		cleanup()

		log.Log.Info("服务已安全关闭")
		os.Exit(0)
	}()
}

// cleanup 清理资源
func cleanup() {
	log.Log.Info("正在保存Raft状态...")
	if err := infra.GetInitManager().PersistRaftState(); err != nil {
		log.Log.Errorf("保存Raft状态失败: %v", err)
	}

	log.Log.Info("正在关闭连接...")
	// 清除客户端缓存
	client.ClearClientCache()
}
