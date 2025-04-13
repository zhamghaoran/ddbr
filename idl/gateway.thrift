namespace go ddbr.rpc.gateway

include "common.thrift"
include "client.thrift"


struct SetRequest {
    1: string key,
    2: string val
    3: common.Password password
}

struct SetResponse {
    1: common.Common common
}

struct GetRequest {
    1: string key
    2: common.Password password
}

struct GetResponse {
    1: string val,
    2: common.Common common
}
struct RegisterSeverReq {
    1: string serverHost
}
struct RegisterSeverResp {
    
}
struct RegisterGatewayResp {
    1: GatewayBasicInfo info
}
struct GatewayBasicInfo {
    1: list<string> severHostSever
    2: common.Password password
}
struct RegisterGatewayReq {

}
struct SetLeaderResp {

}
struct SetLeaderReq {
    1:string leaderHost
}
service Gateway {
    SetResponse Set(1: SetRequest req),
    GetResponse Get(1: GetRequest req),
    RegisterSeverResp RegisterSever(1:  RegisterSeverReq req)
    RegisterGatewayResp RegisterGateway(1: RegisterGatewayReq req)
    SetLeaderResp SetLeader(1: SetLeaderReq req)

}
// kitex -module zhamghaoran/ddbr-gateway -service zhamghaoran/ddbr-gateway  ..\idl\gateway.thrift