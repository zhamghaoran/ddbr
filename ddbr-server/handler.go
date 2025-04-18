package main

import (
	"context"
	"zhamghaoran/ddbr-server/kitex_gen/ddbr/rpc/sever"
	"zhamghaoran/ddbr-server/service"
)

// ServerImpl implements the last service interface defined in the IDL.
type ServerImpl struct{}

// RequestVote implements the ServerImpl interface.
func (s *ServerImpl) RequestVote(ctx context.Context, req *sever.RequestVoteReq) (resp *sever.RequestVoteResp, err error) {
	return service.RequestVote(ctx, req)
}

// AppendEntries implements the ServerImpl interface.
func (s *ServerImpl) AppendEntries(ctx context.Context, req *sever.AppendEntriesReq) (resp *sever.AppendEntriesResp, err error) {
	return service.AppendEntries(ctx, req)
}

// HeartBeat implements the ServerImpl interface.
func (s *ServerImpl) HeartBeat(ctx context.Context, req *sever.HeartbeatReq) (resp *sever.Heartbeatresp, err error) {
	return service.HeartBeat(ctx, req)
}

// JoinCluster implements the ServerImpl interface.
func (s *ServerImpl) JoinCluster(ctx context.Context, req *sever.JoinClusterReq) (resp *sever.JoinClusterResp, err error) {
	// TODO: Your code here...
	return
}

// SyncLogs implements the ServerImpl interface.
func (s *ServerImpl) SyncLogs(ctx context.Context, req *sever.LogSyncReq) (resp *sever.LogSyncResp, err error) {
	return service.SyncLogs(ctx, req)
}

// Set implements the ServerImpl interface for setting key-value.
func (s *ServerImpl) Set(ctx context.Context, req *sever.SetReq) (resp *sever.SetResp, err error) {
	return service.Set(ctx, req)
}

// Get implements the ServerImpl interface for getting a value by key.
func (s *ServerImpl) Get(ctx context.Context, req *sever.GetReq) (resp *sever.GetResp, err error) {
	return service.Get(ctx, req)
}

// Delete implements the ServerImpl interface for deleting a key-value pair.
func (s *ServerImpl) Delete(ctx context.Context, req *sever.DeleteReq) (resp *sever.DeleteResp, err error) {
	return service.Delete(ctx, req)
}
