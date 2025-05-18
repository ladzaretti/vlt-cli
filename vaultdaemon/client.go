package vaultdaemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"syscall"

	"github.com/ladzaretti/vlt-cli/vault/sqlite/vaultcontainer"
	pb "github.com/ladzaretti/vlt-cli/vaultdaemon/cipherdata"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var ErrEmptyVaultPath = errors.New("vault path must not be empty")

// SessionClient wraps the gRPC SessionHandlerClient and provides
// a higher-level interface for session operations.
type SessionClient struct {
	pb      pb.SessionClient
	cleanup func() error
}

func NewSessionClient(client pb.SessionClient, cleanup func() error) *SessionClient {
	return &SessionClient{pb: client, cleanup: cleanup}
}

// Login starts a new session by storing cipher data for the given vault path.
func (c *SessionClient) Login(ctx context.Context, vaultPath string, cipherdata vaultcontainer.CipherData, duration string) error {
	if len(vaultPath) == 0 {
		return ErrEmptyVaultPath
	}

	in := &pb.LoginRequest{
		VaultPath: vaultPath,
		CipherData: &pb.CipherData{
			AuthPhc: cipherdata.AuthPHC,
			KdfPhc:  cipherdata.KDFPHC,
			Nonce:   cipherdata.Nonce,
		},
		Duration: duration,
	}

	_, err := c.pb.Login(ctx, in)

	return err
}

func (c *SessionClient) Logout(ctx context.Context, vaultPath string) error {
	log.Printf("logout request received for vault: %s", vaultPath)

	if len(vaultPath) == 0 {
		return ErrEmptyVaultPath
	}

	_, err := c.pb.Logout(ctx, &pb.SessionRequest{VaultPath: vaultPath})

	return err
}

func (c *SessionClient) GetSession(ctx context.Context, vaultPath string) (*vaultcontainer.CipherData, error) {
	log.Printf("get session request received for vault: %s", vaultPath)

	if len(vaultPath) == 0 {
		return nil, ErrEmptyVaultPath
	}

	session, err := c.pb.GetSession(ctx, &pb.SessionRequest{VaultPath: vaultPath})
	if err != nil {
		return nil, err
	}

	return &vaultcontainer.CipherData{
		AuthPHC: session.GetAuthPhc(),
		KDFPHC:  session.GetKdfPhc(),
		Nonce:   session.GetNonce(),
	}, nil
}

func (c *SessionClient) Close() error {
	return c.cleanup()
}

func Client() (*SessionClient, error) {
	if err := verifySocketSecure(socketPath, os.Getuid()); err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	client := pb.NewSessionClient(conn)
	c := NewSessionClient(client, func() error {
		// return conn.Close()
		return nil
	})

	return c, nil
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
