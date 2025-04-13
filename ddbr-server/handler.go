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
