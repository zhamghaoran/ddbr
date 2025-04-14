package service

import (
	"zhamghaoran/ddbr-gateway/infra"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/common"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/gateway"
	"zhamghaoran/ddbr-gateway/log"
	"zhamghaoran/ddbr-gateway/repo"
)

func CmdService(args []string) error {
	config := infra.GetConfig()
	if config.MasterGatewayHost != infra.Master {
		// 从master 节点拉取配置
		err := GetGatewayInfoFromMaster(config)
		if err != nil {
			log.Log.Errorf("getGatewayInfoFromMaster error: %v", err)
			return err
		}
	} else {
		// 自己就是master，生成配置
		err := repo.InitGatewayInfo(gateway.GatewayBasicInfo{
			SeverHostSever: make([]string, 0),
			Password:       &common.Password{Password: GetPassword()},
		})
		if err != nil {
			return err
		}
	}
	return nil
}
