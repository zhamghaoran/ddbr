package main

import (
	"flag"
	"zhamghaoran/ddbr-server/log"

	"net"
	"zhamghaoran/ddbr-server/infra/initialization"
	ddbr "zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever/server"

	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/server"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	// 初始化资源
	if err := initialization.InitializeResources(*configPath); err != nil {
		log.Log.Errorf("initialization failed: %v", err)

	}

	// 启动服务器
	log.Log.Infof("init ddbr server,port is %s", initialization.GetServerConfig().Port)
	addr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:"+initialization.GetServerConfig().Port)
	code := thrift.NewThriftCodecWithConfig(thrift.FrugalRead | thrift.FrugalWrite)
	svr := ddbr.NewServer(new(ServerImpl), server.WithPayloadCodec(code), server.WithServiceAddr(addr))
	err := svr.Run()
	if err != nil {
		log.Log.Errorf("server failed: %v", err)
	}
}
