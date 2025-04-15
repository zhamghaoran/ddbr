package main

import (
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/server"
	"log"
	"net"
	"os"
	"zhamghaoran/ddbr-gateway/infra"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/gateway/gateway"
	"zhamghaoran/ddbr-gateway/service"
	"zhamghaoran/ddbr-gateway/service/middware"
)

func main() {
	args := os.Args[1:]
	err := infra.ParseConfig(args)
	err = service.CmdService()
	if err != nil {
		panic(err)
	}
	addr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:"+infra.Getport())
	code := thrift.NewThriftCodecWithConfig(thrift.FrugalRead | thrift.FrugalWrite)
	svr := gateway.NewServer(new(GatewayImpl), server.WithMiddleware(middware.AuthorityMiddleware), server.WithServiceAddr(addr),
		server.WithPayloadCodec(code))
	err = svr.Run()
	if err != nil {
		log.Println(err.Error())
	}

}
