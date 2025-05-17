package vaultdaemon

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	pb "github.com/ladzaretti/vlt-cli/vaultdaemon/cipherdata"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

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

func Client() error {
	if err := verifySocketSecure(socketPath, os.Getuid()); err != nil {
		return err
	}

	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer func() { //nolint:wsl
		_ = conn.Close()
	}()

	c := pb.NewSessionHandlerClient(conn)

	_ = c

	// TODO: impl client wrapper

	return nil
}
