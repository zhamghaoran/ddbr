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

	"github.com/go-redis/redis/v8"
)

var (
	// Redis服务器地址
	redisAddr     = flag.String("redis", "localhost:6379", "Redis服务器地址")
	letterRunes   = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	numOperations = flag.Int("n", 10000, "操作次数")
	concurrency   = flag.Int("c", 10, "并发数")
	valueSize     = flag.Int("size", 100, "值大小(字节)")
	testType      = flag.String("type", "set,get", "测试类型：set,get,del")
)

// 随机生成字符串
func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// 连接到Redis单节点
func connectToRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     *redisAddr,
		Password: "", // 如有密码，请设置
		DB:       0,  // 使用默认DB
	})
}

// 测试SET性能
func benchmarkSET(ctx context.Context, client *redis.Client, wg *sync.WaitGroup, numOps int, value string) {
	defer wg.Done()

	for i := 0; i < numOps; i++ {
		key := fmt.Sprintf("benchmark:key:%d", rand.Int())
		err := client.Set(ctx, key, value, 0).Err()
		if err != nil {
			log.Printf("SET操作失败: %v", err)
		}
	}
}

// 测试GET性能
func benchmarkGET(ctx context.Context, client *redis.Client, wg *sync.WaitGroup, keys []string) {
	defer wg.Done()

	for _, key := range keys {
		_, err := client.Get(ctx, key).Result()
		if err != nil && err != redis.Nil {
			log.Printf("GET操作失败: %v", err)
		}
	}
}

// 测试DEL性能
func benchmarkDEL(ctx context.Context, client *redis.Client, wg *sync.WaitGroup, keys []string) {
	defer wg.Done()

	for _, key := range keys {
		err := client.Del(ctx, key).Err()
		if err != nil {
			log.Printf("DEL操作失败: %v", err)
		}
	}
}

func main() {
	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	ctx := context.Background()

	// 连接Redis
	client := connectToRedis()
	defer client.Close()

	// 检查连接
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("连接Redis失败: %v", err)
	}
	fmt.Println("成功连接到Redis")

	testTypes := strings.Split(*testType, ",")
	opsPerRoutine := *numOperations / *concurrency

	// 准备随机值
	value := randomString(*valueSize)

	// 在SET前清理现有数据
	client.FlushAll(ctx)

	// 预先生成用于测试的键列表
	var keys []string
	if contains(testTypes, "set") {
		// 在这里提前生成键名
		keys = make([]string, *numOperations)
		for i := 0; i < *numOperations; i++ {
			keys[i] = fmt.Sprintf("benchmark:key:%d", i)
		}
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

				go benchmarkSET(ctx, client, &wg, endIdx-startIdx, value)
			}

		case "get":
			if !contains(testTypes, "set") {
				// 如果没有执行过SET测试，先填充数据
				fmt.Println("填充数据用于GET测试...")
				for i := 0; i < *numOperations; i++ {
					key := fmt.Sprintf("benchmark:key:%d", i)
					keys = append(keys, key)
					client.Set(ctx, key, value, 0)
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

		case "del":
			if !contains(testTypes, "set") && !contains(testTypes, "get") {
				// 如果没有执行过SET或GET测试，先填充数据
				fmt.Println("填充数据用于DEL测试...")
				for i := 0; i < *numOperations; i++ {
					key := fmt.Sprintf("benchmark:key:%d", i)
					keys = append(keys, key)
					client.Set(ctx, key, value, 0)
				}
			}

			fmt.Printf("开始DEL性能测试: %d次操作, %d并发...\n", *numOperations, *concurrency)
			for i := 0; i < *concurrency; i++ {
				wg.Add(1)
				startIdx := i * opsPerRoutine
				endIdx := startIdx + opsPerRoutine
				if i == *concurrency-1 {
					endIdx = len(keys)
				}

				go benchmarkDEL(ctx, client, &wg, keys[startIdx:endIdx])
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
