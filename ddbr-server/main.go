package main

import (
	"flag"
	"log"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever/server"
	"zhamghaoran/ddbr-server/service"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	// 初始化资源
	if err := service.InitializeResources(*configPath); err != nil {
		log.Fatalf("初始化资源失败: %v", err)
	}

	// 启动服务器
	svr := server.NewServer(new(ServerImpl))
	err := svr.Run()
	if err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
