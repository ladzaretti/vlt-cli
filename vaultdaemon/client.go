package vaultdaemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"syscall"

	pb "github.com/ladzaretti/vlt-cli/vaultdaemon/proto/sessionpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var ErrEmptyVaultPath = errors.New("vault path must not be empty")

// SessionClient wraps the gRPC SessionHandlerClient and provides
// a higher-level interface for session operations.
type SessionClient struct {
	conn *grpc.ClientConn
	pb   pb.SessionClient
}

func NewSessionClient() (*SessionClient, error) {
	if err := verifySocketSecure(socketPath, os.Getuid()); err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	c := &SessionClient{
		conn: conn,
		pb:   pb.NewSessionClient(conn),
	}

	return c, nil
}

// Login starts a new session by storing cipher data for the given vault path.
func (sc *SessionClient) Login(ctx context.Context, vaultPath string, key []byte, nonce []byte, duration string) error {
	if len(vaultPath) == 0 {
		return ErrEmptyVaultPath
	}

	in := &pb.LoginRequest{
		VaultPath: vaultPath,
		Duration:  duration,
		VaultKey: &pb.VaultKey{
			Key:   key,
			Nonce: nonce,
		},
	}

	_, err := sc.pb.Login(ctx, in)

	return err
}

func (sc *SessionClient) Logout(ctx context.Context, vaultPath string) error {
	log.Printf("logout request received for vault: %s", vaultPath)

	if len(vaultPath) == 0 {
		return ErrEmptyVaultPath
	}

	_, err := sc.pb.Logout(ctx, &pb.SessionRequest{VaultPath: vaultPath})

	return err
}

func (sc *SessionClient) GetSessionKey(ctx context.Context, vaultPath string) (key []byte, nonce []byte, _ error) {
	log.Printf("get session request received for vault: %s", vaultPath)

	if len(vaultPath) == 0 {
		return nil, nil, ErrEmptyVaultPath
	}

	vaultKey, err := sc.pb.GetSessionKey(ctx, &pb.SessionRequest{VaultPath: vaultPath})
	if err != nil {
		return nil, nil, err
	}

	return vaultKey.GetKey(), vaultKey.GetNonce(), nil
}

func (sc *SessionClient) Close() error {
	return sc.conn.Close()
}

func verifySocketSecure(path string, uid int) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("could not stat socket: %w", err)
	}

	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("unexpected file stat type")
	}

	if int(stat.Uid) != uid {
		return fmt.Errorf("unexpected socket owner uid: got %d, want %d", stat.Uid, uid)
	}

	if (fi.Mode() & os.ModeSymlink) != 0 {
		return fmt.Errorf("refusing to follow symlink: %s", socketPath)
	}

	if fi.Mode().Perm() != socketPerm {
		return fmt.Errorf("socket file has insecure permissions: %v", fi.Mode().Perm())
	}

	if (fi.Mode() & os.ModeSocket) == 0 {
		return fmt.Errorf("file is not a socket: %s", socketPath)
	}

	return nil
}
