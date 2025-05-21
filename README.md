# Redis与DDBR性能对比测试

这个项目包含了Redis集群的部署配置，以及Redis和DDBR系统的性能测试工具。

## 部署Redis集群

使用Docker Compose部署3主3从的Redis集群：

```bash
# 创建目录结构
mkdir -p redis/master-{1,2,3} redis/slave-{1,2,3}

# 启动Redis集群
docker-compose -f redis-cluster.yml up -d
```

集群初始化会自动完成，将6个Redis节点配置为3主3从的模式。

## 运行Redis性能测试

编译Redis性能测试工具：

```bash
go mod tidy
go build -o redis-benchmark redis-benchmark.go
```

运行测试：

```bash
# 基本测试（默认10000次操作，10并发）
./redis-benchmark

# 自定义测试参数
./redis-benchmark -n 100000 -c 20 -size 1024 -type "set,get,del"
```

支持的参数：
- `-n`: 操作次数，默认10000
- `-c`: 并发数，默认10
- `-size`: 值大小(字节)，默认100
- `-type`: 测试类型，可选"set,get,del"，默认"set,get"

## 运行DDBR系统性能测试

编译DDBR性能测试工具：

```bash
go build -o ddbr-benchmark ddbr-benchmark.go
```

运行测试：

```bash
# 基本测试（默认连接localhost:8080）
./ddbr-benchmark

# 自定义测试参数
./ddbr-benchmark -server "localhost:8080" -n 100000 -c 20 -size 1024 -type "set,get,delete"
```

支持的参数：
- `-server`: DDBR服务器地址，默认"localhost:8080"
- `-n`: 操作次数，默认10000
- `-c`: 并发数，默认10
- `-size`: 值大小(字节)，默认100
- `-type`: 测试类型，可选"set,get,delete"，默认"set,get"

## 性能对比

可以通过运行相同参数的Redis和DDBR测试，对比两个系统的性能：

1. 吞吐量（ops/sec）
2. 响应时间
3. 不同操作类型的性能特点
4. 不同数据大小下的性能变化
5. 并发扩展性能力

## 注意事项

1. 确保本地已安装Go 1.16+和Docker环境
2. 在测试前确保DDBR系统已正确部署
3. Redis集群初始化可能需要一定时间，请等待容器完全启动后再进行测试
4. 比较两个系统性能时，尽量在相同硬件环境和网络条件下进行测试 