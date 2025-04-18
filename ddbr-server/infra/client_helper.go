package infra

import (
	"zhamghaoran/ddbr-server/client"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever/server"
)

// GetLeaderClient 获取Leader客户端的包装函数
func GetLeaderClient(leaderHost string) server.Client {
	return client.GetLeaderClient(leaderHost)
}
