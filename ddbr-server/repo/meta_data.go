package repo

type MetaData struct {
	GatewayHost string
	ServerHost  []string
}

var data MetaData

func SetMetaData(metaData MetaData) {

}
func GetMetaData() MetaData {
	return data
}
