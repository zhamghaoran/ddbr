package main

import (
	"context"
	sever "zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
)

// ServerImpl implements the last service interface defined in the IDL.
type ServerImpl struct{}

// RequestVote implements the ServerImpl interface.
func (s *ServerImpl) RequestVote(ctx context.Context, req *sever.RequestVoteReq) (resp *sever.RequestVoteResp, err error) {
	// TODO: Your code here...
	return
}

// AppendEntries implements the ServerImpl interface.
func (s *ServerImpl) AppendEntries(ctx context.Context, req *sever.AppendEntriesReq) (resp *sever.AppendEntriesResp, err error) {
	// TODO: Your code here...
	return
}

// HeartBeat implements the ServerImpl interface.
func (s *ServerImpl) HeartBeat(ctx context.Context, req *sever.HeartbeatReq) (resp *sever.Heartbeatresp, err error) {
	// TODO: Your code here...
	return
}

// JoinCluster implements the ServerImpl interface.
func (s *ServerImpl) JoinCluster(ctx context.Context, req *sever.JoinClusterReq) (resp *sever.JoinClusterResp, err error) {
	// TODO: Your code here...
	return
}

// SyncLogs implements the ServerImpl interface.
func (s *ServerImpl) SyncLogs(ctx context.Context, req *sever.LogSyncReq) (resp *sever.LogSyncResp, err error) {
	// TODO: Your code here...
	return
}

// Set implements the ServerImpl interface.
func (s *ServerImpl) Set(ctx context.Context, req *sever.SetReq) (resp *sever.SetResp, err error) {
	// TODO: Your code here...
	return
}

// Get implements the ServerImpl interface.
func (s *ServerImpl) Get(ctx context.Context, req *sever.GetReq) (resp *sever.GetResp, err error) {
	// TODO: Your code here...
	return
}

// Delete implements the ServerImpl interface.
func (s *ServerImpl) Delete(ctx context.Context, req *sever.DeleteReq) (resp *sever.DeleteResp, err error) {
	// TODO: Your code here...
	return
}
