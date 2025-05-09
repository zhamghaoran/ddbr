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
    7: bool isPreCommit
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
struct LogSyncReq {
    1: i64 nodeId
    2: i64 lastLogIndex
    3: i64 lastLogTerm
}

struct LogSyncResp {
    1: bool success
    2: string message
    3: list<LogEntry> entries
    4: i64 leaderId
}

struct LogEntry {
    1: i64 term
    2: i64 index
    3: string command
    4: bool preCommitted
    5: bool isRead
}

// 键值存储相关结构体
struct SetReq {
    1: string key
    2: string value
    255: common.Common common
}

struct SetResp {
    1: bool success
    2: string message
    255: common.Common common
}

struct GetReq {
    1: string key
    255: common.Common common
}

struct GetResp {
    1: bool success
    2: string value
    3: bool exists
    255: common.Common common
}

struct DeleteReq {
    1: string key
    255: common.Common common
}

struct DeleteResp {
    1: bool success
    2: string message
    255: common.Common common
}

service Server {
    RequestVoteResp requestVote(1: RequestVoteReq req)
    AppendEntriesResp appendEntries(1: AppendEntriesReq req)
    Heartbeatresp heartBeat(1:HeartbeatReq req)
    JoinClusterResp joinCluster(1: JoinClusterReq req)
    LogSyncResp syncLogs(1: LogSyncReq req)
    // 键值存储操作
    SetResp set(1: SetReq req)
    GetResp get(1: GetReq req)
    DeleteResp delete(1: DeleteReq req)
}
// kitex -module zhamghaoran/ddbr-server  -service zhamghaoran/ddbr-server  ..\idl\server.thrift
// kitex -module zhamghaoran/ddbr-server  -service zhamghaoran/ddbr-server  ..\idl\gateway.thrift