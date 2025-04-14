package repo

type SeverInfo struct {
	severList  []string
	leaderHost string
}

var info SeverInfo

func init() {
	info = SeverInfo{
		severList:  make([]string, 0),
		leaderHost: "",
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
