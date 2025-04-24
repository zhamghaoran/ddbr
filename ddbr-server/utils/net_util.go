package utils

import (
	"context"
	"fmt"
	"github.com/cloudwego/kitex/pkg/utils/kitexutil"
	"net"
	"zhamghaoran/ddbr-server/log"
)

func GetRemoteHost(ctx context.Context) (string, error) {
	cluster, ok := kitexutil.GetCallerAddr(ctx)
	if !ok {
		return "", fmt.Errorf("caller addr error")
	}
	host, _, err := net.SplitHostPort(cluster.String())
	if err != nil {
		return "", err
	}
	log.Log.CtxInfof(ctx, "remote server host is %s", host)
	return host, nil
}
