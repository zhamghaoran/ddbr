package main

import (
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
	// 初始化资源（会调用configs.LoadConfig加载配置）
	if err := infra.InitializeResources(*configPath, *master); err != nil {
		log.Log.Fatalf("初始化资源失败: %v", err)
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

	addr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:"+port)
	code := thrift.NewThriftCodecWithConfig(thrift.FrugalRead | thrift.FrugalWrite)

	// 创建服务实例
	svr := ddbr.NewServer(
		new(ServerImpl),
		server.WithPayloadCodec(code),
		server.WithServiceAddr(addr),
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
