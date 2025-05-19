package vaultdaemon

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/ladzaretti/vlt-cli/vault/sqlite/vaultcontainer"
	pb "github.com/ladzaretti/vlt-cli/vaultdaemon/proto/sessionpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type safeMap[K comparable, V any] struct {
	data map[K]V
	mu   sync.RWMutex
}

func newSafeMap[K comparable, V any]() *safeMap[K, V] {
	return &safeMap[K, V]{data: make(map[K]V)}
}

func (m *safeMap[K, V]) store(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = value
}

//nolint:ireturn
func (m *safeMap[K, V]) load(key K) (value V, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, ok = m.data[key]

	return
}

// Range iterates over all key-value pairs in the map and calls f for each.
//
// Iteration stops if f returns false. The map is write locked for the duration
// of the iteration.
func (m *safeMap[K, V]) Range(f func(K, V) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for k, v := range m.data {
		if !f(k, v) {
			break
		}
	}
}

func (m *safeMap[K, V]) delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
}

type session struct {
	cipherdate vaultcontainer.CipherData
	duration   time.Duration
	done       chan struct{}
}

func newSession(duration time.Duration, cipherdate vaultcontainer.CipherData) *session {
	return &session{
		cipherdate: cipherdate,
		duration:   duration,
		done:       make(chan struct{}),
	}
}

func (s *session) start(cleanup func()) {
	defer cleanup()

	ticker := time.NewTicker(s.duration)
	defer ticker.Stop()

	select {
	case <-ticker.C:
	case <-s.done:
	}
}

func (s *session) stop() {
	select {
	case <-s.done:
		// already closed
	default:
		close(s.done)
	}
}

// sessionServer is used to implement [pb.UnimplementedSessionServer].
type sessionServer struct {
	pb.UnimplementedSessionServer

	sessions *safeMap[string, *session]
}

func newSessionServer() *sessionServer {
	return &sessionServer{
		sessions: newSafeMap[string, *session](),
	}
}

// stopAll stops all active sessions safely via safeMap.
func (s *sessionServer) stopAll() {
	s.sessions.Range(func(_ string, s *session) bool {
		s.stop()
		return true
	})
}

func (s *sessionServer) Login(_ context.Context, req *pb.LoginRequest) (*emptypb.Empty, error) {
	cipherdate := vaultcontainer.CipherData{
		AuthPHC: req.GetCipherData().GetAuthPhc(),
		KDFPHC:  req.GetCipherData().GetKdfPhc(),
		Nonce:   req.GetCipherData().GetNonce(),
	}

	vaultPath := req.GetVaultPath()
	durationStr := req.GetDuration()

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid duration: %v", err)
	}

	session := newSession(duration, cipherdate)
	s.sessions.store(req.GetVaultPath(), session)

	log.Printf("session started for vault: %q: duration %s", vaultPath, durationStr)

	go session.start(func() {
		s.sessions.delete(vaultPath)
		log.Printf("session ended for vault: %s", vaultPath)
	})

	return &emptypb.Empty{}, nil
}

func (s *sessionServer) Logout(_ context.Context, req *pb.SessionRequest) (*emptypb.Empty, error) {
	session, ok := s.sessions.load(req.GetVaultPath())
	if !ok {
		return nil, status.Error(codes.NotFound, "no session found for the given path")
	}

	session.stop()

	s.sessions.delete(req.GetVaultPath())

	return &emptypb.Empty{}, nil
}

func (s *sessionServer) GetSession(_ context.Context, req *pb.SessionRequest) (*pb.CipherData, error) {
	session, ok := s.sessions.load(req.GetVaultPath())
	if !ok {
		return nil, status.Error(codes.NotFound, "no session found for the given path")
	}

	return &pb.CipherData{
		AuthPhc: session.cipherdate.AuthPHC,
		KdfPhc:  session.cipherdate.KDFPHC,
		Nonce:   session.cipherdate.Nonce,
	}, nil
}
