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
	// TODO: Your code here...
	return
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
