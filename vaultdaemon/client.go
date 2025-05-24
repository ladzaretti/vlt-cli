package vaultdaemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	pb "github.com/ladzaretti/vlt-cli/vaultdaemon/proto/sessionpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	ErrEmptyVaultPath    = errors.New("vault path must not be empty")
	ErrSocketUnavailable = errors.New("vault daemon socket unavailable")
)

// SessionClient wraps the gRPC SessionHandlerClient and provides
// a higher-level interface for session operations.
type SessionClient struct {
	conn *grpc.ClientConn
	pb   pb.SessionClient
}

// NewSessionClient connects to the local vault daemon over a UNIX socket
// and returns a SessionClient.
//
// It returns [ErrSocketUnavailable] if the daemon socket is missing or inaccessible.
func NewSessionClient() (*SessionClient, error) {
	if err := verifySocketSecure(socketPath, os.Getuid()); err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to connect: %v", ErrSocketUnavailable, err)
	}

	c := &SessionClient{
		conn: conn,
		pb:   pb.NewSessionClient(conn),
	}

	return c, nil
}

// Login starts a new session by storing cipher data for the given vault path.
func (sc *SessionClient) Login(ctx context.Context, vaultPath string, key []byte, nonce []byte, duration time.Duration) error {
	if sc == nil {
		return nil
	}

	if len(vaultPath) == 0 {
		return ErrEmptyVaultPath
	}

	in := &pb.LoginRequest{
		VaultPath:       vaultPath,
		DurationSeconds: int64(duration.Seconds()),
		VaultKey: &pb.VaultKey{
			Key:   key,
			Nonce: nonce,
		},
	}

	_, err := sc.pb.Login(ctx, in)

	return err
}

// Logout requests the daemon to clear the session for the given vault path.
func (sc *SessionClient) Logout(ctx context.Context, vaultPath string) error {
	if sc == nil {
		return nil
	}

	if len(vaultPath) == 0 {
		return ErrEmptyVaultPath
	}

	_, err := sc.pb.Logout(ctx, &pb.SessionRequest{VaultPath: vaultPath})

	return err
}

// GetSessionKey retrieves the session key and nonce for the given vault path.
func (sc *SessionClient) GetSessionKey(ctx context.Context, vaultPath string) (key []byte, nonce []byte, _ error) {
	if sc == nil {
		return nil, nil, nil
	}

	if len(vaultPath) == 0 {
		return nil, nil, ErrEmptyVaultPath
	}

	vaultKey, err := sc.pb.GetSessionKey(ctx, &pb.SessionRequest{VaultPath: vaultPath})
	if err != nil {
		return nil, nil, err
	}

	return vaultKey.GetKey(), vaultKey.GetNonce(), nil
}

// Close safely shuts down the gRPC connection.
// No-op if the client or connection is nil.
func (sc *SessionClient) Close() error {
	if sc == nil || sc.conn == nil {
		return nil
	}

	return sc.conn.Close()
}

func verifySocketSecure(path string, uid int) (retErr error) {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%w: could not stat socket: %w", ErrSocketUnavailable, err)
	}

	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("socket verify: unexpected file stat type")
	}

	if int(stat.Uid) != uid {
		return fmt.Errorf("socket verify: unexpected socket owner uid: got %d, want %d", stat.Uid, uid)
	}

	if (fi.Mode() & os.ModeSymlink) != 0 {
		return fmt.Errorf("socket verify: refusing to follow symlink: %s", path)
	}

	if (fi.Mode() & os.ModeSocket) == 0 {
		return fmt.Errorf("socket verify: file is not a socket: %s", path)
	}

	if fi.Mode().Perm() != socketPerm {
		return fmt.Errorf("socket verify: socket file has insecure permissions: %v", fi.Mode().Perm())
	}

	return nil
}
