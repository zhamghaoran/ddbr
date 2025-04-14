package constants

import "errors"

var (
	// GatewayBasicInfo error
	PasswordInfoParamError = errors.New("passwordInfo param error")
	SeverHostParamError    = errors.New("severHost param error")
)
