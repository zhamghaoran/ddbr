package log

import "github.com/cloudwego/kitex/pkg/klog"

var Log klog.FullLogger

func init() {
	Log = klog.DefaultLogger()
}
