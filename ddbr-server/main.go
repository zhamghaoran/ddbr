package main

import (
	"flag"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/server"
	"log"
	"net"
	ddbr "zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever/server"
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
	addr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:"+service.GetServerConfig().Port)
	code := thrift.NewThriftCodecWithConfig(thrift.FrugalRead | thrift.FrugalWrite)
	svr := ddbr.NewServer(new(ServerImpl), server.WithPayloadCodec(code), server.WithServiceAddr(addr))
	err := svr.Run()
	if err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
