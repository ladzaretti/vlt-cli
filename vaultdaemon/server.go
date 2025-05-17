package vaultdaemon

import (
	"context"

	pb "github.com/ladzaretti/vlt-cli/vaultdaemon/cipherdata"
)

// server is used to implement [pb.SessionHandlerServer].
type server struct {
	pb.UnimplementedSessionHandlerServer
}

func (*server) GetSession(context.Context, *pb.SessionRequest) (*pb.CipherData, error) {
	return nil, nil
}

func (*server) Login(context.Context, *pb.LoginRequest) (*pb.Empty, error) {
	return nil, nil
}

func (*server) Logout(context.Context, *pb.SessionRequest) (*pb.Empty, error) {
	return nil, nil
}
