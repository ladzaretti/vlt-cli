package vaultdaemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/ladzaretti/vlt-cli/vaultdaemon/cipherdata"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

// socketPerm defines the file permission mode
// for the unix domain socket.
const socketPerm = 0o600

// socketPath is the location of the unix domain socket
// used by the daemon.
var socketPath = fmt.Sprintf("/run/user/%d/vlt.sock", os.Getuid())

// getCred returns the credentials from the remote end of a unix socket.
func getCred(conn net.Conn) (*unix.Ucred, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("connection is not a *net.UnixConn: got %T", conn)
	}

	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return nil, err
	}

	var (
		ucred    *unix.Ucred
		ucredErr error
	)

	err = rawConn.Control(func(fd uintptr) {
		// Getsockopt syscall to retrieve peer credentials (uid, gid, pid)
		// from the remote end of the connected unix socket
		//
		// https://man7.org/linux/man-pages/man7/unix.7.html (SO_PEERCRED details)
		ucred, ucredErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if err != nil {
		return nil, err
	}

	if ucredErr != nil {
		return nil, ucredErr
	}

	return ucred, nil
}

// uidCheckingListener wraps a [net.Listener] and only accepts connections
// from clients matching the allowed UID.
type uidCheckingListener struct {
	net.Listener
	allowedUID int
}

// Accept returns the next connection if the client's UID matches allowedUID.
// Other connections are closed and skipped.
func (l *uidCheckingListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		ucred, err := getCred(conn)
		if err != nil {
			log.Printf("uid check failed: %v", err)
			_ = conn.Close() //nolint:wsl

			continue
		}

		if int(ucred.Uid) != l.allowedUID {
			log.Printf("connection from disallowed uid: %d", ucred.Uid)
			_ = conn.Close() //nolint:wsl

			continue
		}

		// connection allowed
		return conn, nil
	}
}

// Run starts the vltd daemon and serves gRPC over a UNIX domain socket.
//
// It creates the socket with 0600 permissions and only allows connections
// from the current user, validated by UID.
func Run() {
	log.SetPrefix("[vltd] ")

	log.Printf("daemon started")

	_ = os.Remove(socketPath) // remove stale socket

	socket, err := net.Listen("unix", socketPath)
	if err != nil {
		panic(fmt.Errorf("unix socket listen: %w", err))
	}
	defer func() { //nolint:wsl
		_ = socket.Close()
		_ = os.Remove(socketPath)
	}()

	if err := os.Chmod(socketPath, socketPerm); err != nil {
		panic(fmt.Errorf("unix socket chmod: %w", err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv := grpc.NewServer()
	handler := newSessionHandler()

	pb.RegisterSessionServer(srv, handler)

	lis := &uidCheckingListener{
		Listener:   socket,
		allowedUID: os.Getuid(),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		log.Printf("server listening at: %v", socket.Addr())

		if err := srv.Serve(lis); err != nil {
			log.Printf("grpc server stopped with error: %v", err)
			return
		}

		log.Printf("grpc server stopped")
	}()

	<-ctx.Done()

	log.Printf("received shutdown signal: shutting down...")

	srv.Stop()
	handler.stopAll()

	<-done
	log.Println("shutdown complete")
}
