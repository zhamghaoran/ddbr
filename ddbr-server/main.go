package main

import (
	"flag"
	"net"
	"zhamghaoran/ddbr-server/infra"
	ddbr "zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever/server"
	"zhamghaoran/ddbr-server/log"

	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/server"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "", "配置文件路径")
	isMaster := flag.Bool("master", false, "指定master")
	flag.Parse()

	// 初始化资源
	if err := infra.InitializeResources(*configPath, *isMaster); err != nil {
		log.Log.Errorf("initialization failed: %v", err)
	}
	// 启动服务器
	log.Log.Infof("init ddbr server,port is %s", infra.GetServerConfig().Port)
	addr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:"+infra.GetServerConfig().Port)
	code := thrift.NewThriftCodecWithConfig(thrift.FrugalRead | thrift.FrugalWrite)
	svr := ddbr.NewServer(new(ServerImpl), server.WithPayloadCodec(code), server.WithServiceAddr(addr))
	err := svr.Run()
	if err != nil {
		log.Log.Errorf("server failed: %v", err)
	}
}
