package repo

type SeverInfo struct {
	severList  []string
	leaderHost string
}
type GatewayInfo struct {
	gatewayHost map[string]struct{}
}

var gatewayInfo GatewayInfo
var info SeverInfo

func init() {
	info = SeverInfo{
		severList:  make([]string, 0),
		leaderHost: "",
	}
	gatewayInfo = GatewayInfo{
		gatewayHost: make(map[string]struct{}),
	}
}

func AddServer(host string) {
	info.severList = append(info.severList, host)
}
func SetLeader(host string) {
	info.leaderHost = host
}
func GetSeverList() []string {
	return info.severList
}
func GetLeaderHost() string {
	return info.leaderHost
}
func AddGatewayHost(host string) {
	gatewayInfo.gatewayHost[host] = struct{}{}
}
func GetGatewayList() []string {
	gatewayList := make([]string, 0)
	for k, _ := range gatewayInfo.gatewayHost {
		gatewayList = append(gatewayList, k)
	}
	return gatewayList
}
