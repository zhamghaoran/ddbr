package repo

import (
	"zhamghaoran/ddbr-gateway/constants"
	"zhamghaoran/ddbr-gateway/kitex_gen/ddbr/rpc/gateway"
	"zhamghaoran/ddbr-gateway/log"
)

var GatewayBasicInfo gateway.GatewayBasicInfo

func InitGatewayInfo(info gateway.GatewayBasicInfo) error {
	if err := paramCheck(info); err != nil {
		log.Log.Errorf("InitGatewayInfo error: %v", err)
		return err
	}
	GatewayBasicInfo = info

	return nil
}
func paramCheck(info gateway.GatewayBasicInfo) error {
	if info.Password == nil {
		return constants.PasswordInfoParamError
	}
	//if len(info.SeverHostSever) == 0 {
	//	return constants.SeverHostParamError
	//}
	return nil
}
