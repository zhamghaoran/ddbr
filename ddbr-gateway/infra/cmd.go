package infra

import (
	"errors"
)

const (
	GateWayHostConfig = "-gateway"
	Port              = "-port"
	Master            = "master"
)

type CmdConfig struct {
	MasterGatewayHost string
	port              string
}

var config *CmdConfig

func ParseConfig(args []string) error {
	if len(args)%2 != 0 {
		return errors.New("invalid cmd len")
	}
	configMap := make(map[string]string)
	for i := 0; i < len(args); i++ {
		configMap[args[i]] = args[i+1]
		i++
	}
	parse(configMap)
	return nil
}
func parse(configMap map[string]string) *CmdConfig {
	config = &CmdConfig{}
	for k, v := range configMap {
		switch k {
		case GateWayHostConfig:
			config.MasterGatewayHost = v
		case Port:
			config.port = v
		}
	}
	return config
}
func GetConfig() *CmdConfig {
	return config
}
func Getport() string {
	return config.port
}
