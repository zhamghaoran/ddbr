package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/common"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
)

var (
	// DDBR集群地址
	ddbrsrvAddr   = flag.String("server", "localhost:8080", "DDBR服务器地址")
	letterRunes   = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	numOperations = flag.Int("n", 10000, "操作次数")
	concurrency   = flag.Int("c", 10, "并发数")
	valueSize     = flag.Int("size", 100, "值大小(字节)")
	testType      = flag.String("type", "set,get", "测试类型：set,get,delete")
)

// 随机生成字符串
func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// 连接DDBR服务器
func connectToDDBR(addr string) *client.LeaderClient {
	return client.GetLeaderClient(addr)
}

// 测试SET性能
func benchmarkSET(ctx context.Context, client *client.LeaderClient, wg *sync.WaitGroup, keys []string, value string) {
	defer wg.Done()

	for _, key := range keys {
		req := &sever.SetReq{
			Key:    key,
			Value:  value,
			Common: &common.Common{},
		}

		_, err := client.Set(ctx, req)
		if err != nil {
			log.Printf("SET操作失败 (key=%s): %v", key, err)
		}
	}
}

// 测试GET性能
func benchmarkGET(ctx context.Context, client *client.LeaderClient, wg *sync.WaitGroup, keys []string) {
	defer wg.Done()

	for _, key := range keys {
		req := &sever.GetReq{
			Key:    key,
			Common: &common.Common{},
		}

		_, err := client.Get(ctx, req)
		if err != nil {
			log.Printf("GET操作失败 (key=%s): %v", key, err)
		}
	}
}

// 测试DELETE性能
func benchmarkDELETE(ctx context.Context, client *client.LeaderClient, wg *sync.WaitGroup, keys []string) {
	defer wg.Done()

	for _, key := range keys {
		req := &sever.DeleteReq{
			Key:    key,
			Common: &common.Common{},
		}

		_, err := client.Delete(ctx, req)
		if err != nil {
			log.Printf("DELETE操作失败 (key=%s): %v", key, err)
		}
	}
}

func main() {
	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	ctx := context.Background()

	// 连接DDBR服务器
	client := connectToDDBR(*ddbrsrvAddr)
	if client == nil {
		log.Fatalf("连接DDBR服务器失败")
	}
	fmt.Println("成功连接到DDBR服务器")

	testTypes := strings.Split(*testType, ",")
	opsPerRoutine := *numOperations / *concurrency

	// 准备随机值
	value := randomString(*valueSize)

	// 预先生成用于测试的键列表
	keys := make([]string, *numOperations)
	for i := 0; i < *numOperations; i++ {
		keys[i] = fmt.Sprintf("benchmark:key:%d", i)
	}

	// 执行性能测试
	for _, test := range testTypes {
		test = strings.TrimSpace(test)

		var wg sync.WaitGroup
		start := time.Now()

		switch test {
		case "set":
			fmt.Printf("开始SET性能测试: %d次操作, %d并发...\n", *numOperations, *concurrency)
			for i := 0; i < *concurrency; i++ {
				wg.Add(1)
				startIdx := i * opsPerRoutine
				endIdx := startIdx + opsPerRoutine
				if i == *concurrency-1 {
					endIdx = *numOperations
				}

				go benchmarkSET(ctx, client, &wg, keys[startIdx:endIdx], value)
			}

		case "get":
			// 确保数据已经写入
			if !contains(testTypes, "set") {
				fmt.Println("填充数据用于GET测试...")
				for _, key := range keys[:100] { // 仅填充部分数据进行测试
					req := &sever.SetReq{
						Key:    key,
						Value:  value,
						Common: &common.Common{},
					}
					client.Set(ctx, req)
				}
			}

			fmt.Printf("开始GET性能测试: %d次操作, %d并发...\n", *numOperations, *concurrency)
			for i := 0; i < *concurrency; i++ {
				wg.Add(1)
				startIdx := i * opsPerRoutine
				endIdx := startIdx + opsPerRoutine
				if i == *concurrency-1 {
					endIdx = *numOperations
				}

				go benchmarkGET(ctx, client, &wg, keys[startIdx:endIdx])
			}

		case "delete":
			// 确保数据已经写入
			if !contains(testTypes, "set") && !contains(testTypes, "get") {
				fmt.Println("填充数据用于DELETE测试...")
				for _, key := range keys[:100] { // 仅填充部分数据进行测试
					req := &sever.SetReq{
						Key:    key,
						Value:  value,
						Common: &common.Common{},
					}
					client.Set(ctx, req)
				}
			}

			fmt.Printf("开始DELETE性能测试: %d次操作, %d并发...\n", *numOperations, *concurrency)
			for i := 0; i < *concurrency; i++ {
				wg.Add(1)
				startIdx := i * opsPerRoutine
				endIdx := startIdx + opsPerRoutine
				if i == *concurrency-1 {
					endIdx = *numOperations
				}

				go benchmarkDELETE(ctx, client, &wg, keys[startIdx:endIdx])
			}
		}

		wg.Wait()
		elapsed := time.Since(start)

		opsPerSecond := float64(*numOperations) / elapsed.Seconds()
		fmt.Printf("%s测试完成: %.2f ops/sec (总耗时: %s)\n", test, opsPerSecond, elapsed)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.TrimSpace(s) == item {
			return true
		}
	}
	return false
}
