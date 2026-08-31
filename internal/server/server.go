package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"distributed-cache/internal/cache"
)

// Server exposes a cache.Cache over TCP using a small RESP-flavored inline
// protocol. ListenAndServe runs the accept loop and spawns one goroutine per
// connection; Shutdown closes the listener and drains in-flight connections.
type Server struct {
	cache  *cache.Cache
	logger *slog.Logger

	mu       sync.Mutex
	listener net.Listener

	wg sync.WaitGroup // in-flight connections

	commands    atomic.Int64 // total commands processed
	activeConns atomic.Int64 // connections currently open
	errors      atomic.Int64 // command + I/O errors

	done         chan struct{} // closed once shutdown completes
	shutdownOnce sync.Once
}

// New returns a Server serving the given cache. A nil logger falls back to
// slog.Default.
func New(c *cache.Cache, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		cache:  c,
		logger: logger,
		done:   make(chan struct{}),
	}
}

// Addr returns the bound listener address, or nil before ListenAndServe has
// bound. Useful for tests that listen on :0.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return nil
	}
	return s.listener.Addr()
}

// ListenAndServe binds addr and accepts connections until the listener is
// closed by Shutdown. It returns nil after a graceful close and a non-nil
// error if it cannot bind.
func (s *Server) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.listener = ln
	s.mu.Unlock()

	s.logger.Info("server listening", "addr", ln.Addr().String())

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil // graceful shutdown
			}
			s.errors.Add(1)
			s.logger.Error("accept failed", "err", err)
			continue
		}
		s.wg.Add(1)
		s.activeConns.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer s.activeConns.Add(-1)
	defer conn.Close()

	remote := conn.RemoteAddr().String()
	s.logger.Debug("connection opened", "remote", remote)
	defer s.logger.Debug("connection closed", "remote", remote)

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if !errors.Is(err, io.EOF) {
				s.errors.Add(1)
				s.logger.Debug("read error", "remote", remote, "err", err)
			}
			return
		}

		args := parseCommand(line)
		if len(args) == 0 {
			continue // skip blank lines
		}

		s.commands.Add(1)
		closeConn, cmdErr := s.dispatch(w, args)
		if cmdErr != nil {
			s.errors.Add(1)
			s.logger.Debug("command error", "remote", remote, "err", cmdErr)
		}
		if err := w.Flush(); err != nil {
			s.errors.Add(1)
			s.logger.Debug("write error", "remote", remote, "err", err)
			return
		}
		if closeConn {
			return
		}
	}
}

// Shutdown closes the listener (unblocking Accept and rejecting new
// connections), then waits for in-flight connections to drain, bounded by ctx.
// It is idempotent: the first caller's result is reused by later calls.
func (s *Server) Shutdown(ctx context.Context) error {
	var shutdownErr error
	s.shutdownOnce.Do(func() {
		s.mu.Lock()
		ln := s.listener
		s.mu.Unlock()
		if ln != nil {
			_ = ln.Close()
		}

		drained := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(drained)
		}()

		select {
		case <-drained:
		case <-ctx.Done():
			shutdownErr = ctx.Err()
		}

		s.logger.Info("server shutdown",
			"commands", s.commands.Load(),
			"active_connections", s.activeConns.Load(),
			"errors", s.errors.Load(),
		)
		close(s.done)
	})
	return shutdownErr
}
