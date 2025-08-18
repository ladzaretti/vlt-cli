package vaultdaemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/ladzaretti/vlt-cli/vaultdaemon/proto/sessionpb"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

// socketPerm is the file permission mode for the unix domain socket.
const socketPerm = 0o600

// socketPath is the path of the unix domain socket
// used by the daemon.
var socketPath = fmt.Sprintf("/run/user/%d/vlt.sock", os.Getuid())

// Run starts the vltd daemon and serves grpc over a unix domain socket
// that only allows connections from the same user that runs the daemon.
func Run(ctx context.Context) error {
	log.SetPrefix("[vltd] ")

	log.Print("daemon started")

	if socketInUse(ctx, socketPath) {
		return fmt.Errorf("socket already in use: %v", socketPath)
	}

	_ = os.Remove(socketPath) // remove stale socket

	var lc net.ListenConfig

	socket, err := lc.Listen(ctx, "unix", socketPath)
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
	handler := newSessionServer()

	pb.RegisterSessionServer(srv, handler)

	lis := &secureUnixListener{
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

		log.Print("grpc server stopped")
	}()

	<-ctx.Done()

	log.Print("received shutdown signal: shutting down...")

	srv.Stop()
	handler.stopAll()

	<-done
	log.Println("shutdown complete")

	return ctx.Err()
}

func socketInUse(ctx context.Context, path string) bool {
	var d net.Dialer

	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return false
	}
	_ = conn.Close() //nolint:wsl

	return true
}

// secureUnixListener wraps a unix [net.Listener] and only accepts connections
// from clients matching the allowed uid.
type secureUnixListener struct {
	net.Listener
	allowedUID int
}

// Accept only returns the next connection if the client's uid matches [secureUnixListener.allowedUID].
// Other connections are closed and skipped.
func (l *secureUnixListener) Accept() (net.Conn, error) {
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
		// see SO_PEERCRED:
		// https://man7.org/linux/man-pages/man7/unix.7.html
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
