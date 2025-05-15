package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/transport"
	"time"
	"zhamghaoran/ddbr-client/kitex_gen/ddbr/rpc/common"
	"zhamghaoran/ddbr-client/kitex_gen/ddbr/rpc/gateway"
	gw "zhamghaoran/ddbr-client/kitex_gen/ddbr/rpc/gateway/gateway"

	"github.com/cloudwego/kitex/client"
)

// 引入gateway客户端代码
// 首先需要运行命令生成：
// kitex -module zhamghaoran/ddbr-client ../idl/gateway.thrift

// ClientManager 管理网关客户端连接
type ClientManager struct {
	gatewayClient gw.Client
	gatewayAddr   string
}

// NewClientManager 创建客户端管理器
func NewClientManager(gatewayAddr string) gw.Client {
	// 创建Kitex客户端

	c := gw.MustNewClient("gateway", client.WithHostPorts(gatewayAddr), client.WithPayloadCodec(
		thrift.NewThriftCodecWithConfig(thrift.FrugalRead|thrift.FrugalWrite),
	), client.WithTransportProtocol(transport.Framed))

	fmt.Printf("初始化网关RPC客户端连接: %s\n", gatewayAddr)
	return c
}

func main() {
	// 命令行参数
	gatewayAddr := "localhost:8080"

	flag.Parse()

	// 初始化客户端管理器
	clientManager := NewClientManager(gatewayAddr)
	fmt.Printf("连接到网关: %s\n", gatewayAddr)

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	password := "799972173318"
	respSet, err := clientManager.Set(ctx, &gateway.SetRequest{
		Key:      "init_key1",
		Val:      "test1",
		Password: &common.Password{Password: password},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("set resp is %+v\n", respSet)
	resp, err := clientManager.Get(ctx, &gateway.GetRequest{
		Key:      "init_key1",
		Password: &common.Password{Password: password},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("第一次get %+v\n", resp)
	_, err = clientManager.Delete(ctx, &gateway.DeleteRequest{
		Key:      "init_key1",
		Password: &common.Password{Password: password},
	})
	if err != nil {
		panic(err)
	}
	resp1, err := clientManager.Get(ctx, &gateway.GetRequest{
		Key:      "init_key1",
		Password: &common.Password{Password: password},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("第二次get%+v\n", resp1)

}
