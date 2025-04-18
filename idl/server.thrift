namespace go ddbr.rpc.sever
include "common.thrift"

struct RequestVoteReq {
    1: i64 term
    2: i64 candidateId
    3: i64 lastLogIndex
    4: i64 lastLogTerm
}
struct RequestVoteResp {
    1: i64 term
    2: bool voteGranted
    3: common.Common common
}
struct AppendEntriesReq {
    1: i64 term
    2: i64 leaderId
    3: i64 prevLogIndex
    4: i64 prevLogTerm
    5: list<string> entries
    6: i64 leaderCommit
}
struct AppendEntriesResp {
    1: i64 term
    2: bool success
}
struct HeartbeatReq {

}
struct Heartbeatresp{
    1: list<string> Peers
    255: common.Common common
}
struct JoinClusterReq {

}
struct JoinClusterResp {

}
service Server {
    RequestVoteResp requestVote(1: RequestVoteReq req)
    AppendEntriesResp appendEntries(1:AppendEntriesReq req)
    Heartbeatresp heartBeat(1:HeartbeatReq req)
    JoinClusterResp joinCluster(1: JoinClusterReq req)
}
// kitex -module zhamghaoran/ddbr-server  -service zhamghaoran/ddbr-server  ..\idl\server.thrift
// kitex -module zhamghaoran/ddbr-server  -service zhamghaoran/ddbr-server  ..\idl\gateway.thrift