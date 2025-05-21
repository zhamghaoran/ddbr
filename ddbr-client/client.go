package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/transport"
	"github.com/go-redis/redis/v8"
	"math/rand"
	"strconv"
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

func connectToRedisCluster() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis服务器地址
		Password: "",               // 无密码
		DB:       0,
	})

}
func main() {
	// 命令行参数
	gatewayAddr := "localhost:8080"

	flag.Parse()

	// 初始化客户端管理器
	clientManager := NewClientManager(gatewayAddr)
	fmt.Printf("连接到网关: %s\n", gatewayAddr)

	// 创建上下文
	ctx := context.Background()
	password := "235254608168"
	// 初始化48 个key
	for i := 0; i <= 4096; i++ {
		_, _ = clientManager.Set(ctx, &gateway.SetRequest{
			Key:      "init_key" + strconv.Itoa(i),
			Val:      "test1",
			Password: &common.Password{Password: password},
		})
	}
	beginTime := time.Now()
	// 测试读写100w 次，统计耗时
	for i := 1; i <= 100000; i++ {
		_, err := clientManager.Get(ctx, &gateway.GetRequest{
			Key:      "init_key" + strconv.Itoa(rand.Intn(48)),
			Password: &common.Password{Password: password},
		})
		if err != nil {
			fmt.Println(err.Error())
		}
	}
	costTime := time.Since(beginTime)
	fmt.Printf("ddbr_costTime: %s\n", costTime)
	redisClient := connectToRedisCluster()
	for i := 0; i <= 4096; i++ {
		if err := redisClient.Set(ctx, "init_key"+strconv.Itoa(i), "test1", -1).Err(); err != nil {
			fmt.Printf("err is :%v", err.Error())
		}
	}
	redisStartTime := time.Now()
	for i := 1; i <= 100000; i++ {
		if err := redisClient.Get(ctx, "init_key"+strconv.Itoa(rand.Intn(48))).Err(); err != nil {
			fmt.Printf("err is :%v", err.Error())
		}
	}
	costTime = time.Since(redisStartTime)
	fmt.Printf("redis_costTime: %s\n", costTime)
}
