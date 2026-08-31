package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"distributed-cache/internal/cache"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestServer starts a server on 127.0.0.1:0 and returns the cache, the
// server, and its dialable address. Cleanup shuts the server down.
func newTestServer(t *testing.T, maxEntries int) (*cache.Cache, *Server, string) {
	t.Helper()
	c, err := cache.NewCache(maxEntries)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	srv := New(c, testLogger())

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe("127.0.0.1:0") }()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		select {
		case <-errCh:
		case <-time.After(2 * time.Second):
			t.Error("server did not stop after shutdown")
		}
		c.Close()
	})

	deadline := time.Now().Add(2 * time.Second)
	for srv.Addr() == nil {
		if time.Now().After(deadline) {
			t.Fatal("server did not start listening in time")
		}
		time.Sleep(time.Millisecond)
	}
	return c, srv, srv.Addr().String()
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func send(t *testing.T, conn net.Conn, cmd string) {
	t.Helper()
	if _, err := conn.Write([]byte(cmd)); err != nil {
		t.Fatalf("write %q: %v", cmd, err)
	}
}

// readReplyErr reads one RESP-flavored reply and returns a normalized string:
// the simple-string/integer payload, "<nil>" for a null bulk, the bulk payload,
// or the raw "-ERR ..." line.
func readReplyErr(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")

	switch {
	case strings.HasPrefix(line, "+"):
		return line[1:], nil
	case strings.HasPrefix(line, ":"):
		return line[1:], nil
	case strings.HasPrefix(line, "-"):
		return line, nil
	case line == "$-1":
		return "<nil>", nil
	case strings.HasPrefix(line, "$"):
		n, err := strconv.Atoi(line[1:])
		if err != nil {
			return "", fmt.Errorf("bad bulk length %q: %w", line, err)
		}
		body := make([]byte, n+2) // payload + trailing CRLF
		if _, err := io.ReadFull(r, body); err != nil {
			return "", err
		}
		return string(body[:n]), nil
	default:
		return "", fmt.Errorf("unexpected reply line %q", line)
	}
}

func readReply(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	got, err := readReplyErr(r)
	if err != nil {
		t.Fatalf("read reply: %v", err)
	}
	return got
}

func TestServerPing(t *testing.T) {
	_, _, addr := newTestServer(t, 10)
	conn := dial(t, addr)
	r := bufio.NewReader(conn)

	send(t, conn, "PING\r\n")
	if got := readReply(t, r); got != "PONG" {
		t.Fatalf("PING = %q; want PONG", got)
	}
}

func TestServerSetGet(t *testing.T) {
	c, _, addr := newTestServer(t, 10)
	conn := dial(t, addr)
	r := bufio.NewReader(conn)

	send(t, conn, "SET foo bar\r\n")
	if got := readReply(t, r); got != "OK" {
		t.Fatalf("SET = %q; want OK", got)
	}
	send(t, conn, "GET foo\r\n")
	if got := readReply(t, r); got != "bar" {
		t.Fatalf("GET foo = %q; want bar", got)
	}

	if v, ok := c.Get("foo"); !ok || v != "bar" {
		t.Fatalf("cache.Get(foo) = %v, %v; want bar, true", v, ok)
	}
}

func TestServerGetMiss(t *testing.T) {
	_, _, addr := newTestServer(t, 10)
	conn := dial(t, addr)
	r := bufio.NewReader(conn)

	send(t, conn, "GET nope\r\n")
	if got := readReply(t, r); got != "<nil>" {
		t.Fatalf("GET nope = %q; want <nil>", got)
	}
}

func TestServerDel(t *testing.T) {
	_, _, addr := newTestServer(t, 10)
	conn := dial(t, addr)
	r := bufio.NewReader(conn)

	send(t, conn, "DEL x\r\n")
	if got := readReply(t, r); got != "0" {
		t.Fatalf("DEL missing = %q; want 0", got)
	}

	send(t, conn, "SET x 1\r\n")
	if got := readReply(t, r); got != "OK" {
		t.Fatalf("SET x = %q; want OK", got)
	}
	send(t, conn, "DEL x\r\n")
	if got := readReply(t, r); got != "1" {
		t.Fatalf("DEL x = %q; want 1", got)
	}
	send(t, conn, "DEL x\r\n")
	if got := readReply(t, r); got != "0" {
		t.Fatalf("DEL x again = %q; want 0", got)
	}
}

func TestServerSetEX(t *testing.T) {
	_, _, addr := newTestServer(t, 10)
	conn := dial(t, addr)
	r := bufio.NewReader(conn)

	send(t, conn, "SET k v EX 1\r\n")
	if got := readReply(t, r); got != "OK" {
		t.Fatalf("SET EX = %q; want OK", got)
	}
	send(t, conn, "GET k\r\n")
	if got := readReply(t, r); got != "v" {
		t.Fatalf("GET k before expiry = %q; want v", got)
	}

	time.Sleep(1100 * time.Millisecond)

	send(t, conn, "GET k\r\n")
	if got := readReply(t, r); got != "<nil>" {
		t.Fatalf("GET k after expiry = %q; want <nil>", got)
	}
}

func TestServerErrors(t *testing.T) {
	_, _, addr := newTestServer(t, 10)
	conn := dial(t, addr)
	r := bufio.NewReader(conn)

	tests := []struct {
		cmd     string
		wantErr string // substring
	}{
		{"NOPE\r\n", "unknown command"},
		{"GET\r\n", "wrong number of arguments"},
		{"SET onlykey\r\n", "wrong number of arguments"},
		{"SET k v NX 5\r\n", "syntax error"},
		{"SET k v EX abc\r\n", "invalid expire time"},
		{"SET k v EX 0\r\n", "invalid expire time"},
	}
	for _, tt := range tests {
		send(t, conn, tt.cmd)
		got := readReply(t, r)
		if !strings.Contains(got, tt.wantErr) {
			t.Fatalf("%q => %q; want error containing %q", strings.TrimSpace(tt.cmd), got, tt.wantErr)
		}
	}
}

func TestServerCaseInsensitive(t *testing.T) {
	_, _, addr := newTestServer(t, 10)
	conn := dial(t, addr)
	r := bufio.NewReader(conn)

	send(t, conn, "ping\r\n")
	if got := readReply(t, r); got != "PONG" {
		t.Fatalf("ping = %q; want PONG", got)
	}
}

func TestServerStats(t *testing.T) {
	_, srv, addr := newTestServer(t, 10)
	conn := dial(t, addr)
	r := bufio.NewReader(conn)

	send(t, conn, "PING\r\n")
	_ = readReply(t, r)

	send(t, conn, "STATS\r\n")
	stats := readReply(t, r)
	for _, want := range []string{"commands:", "active_connections:", "errors:"} {
		if !strings.Contains(stats, want) {
			t.Fatalf("STATS = %q; want substring %q", stats, want)
		}
	}
	// The PING we just sent must be counted (plus this STATS itself).
	if got := srv.commands.Load(); got < 2 {
		t.Fatalf("commands counter = %d; want >= 2", got)
	}
}

func TestServerQuit(t *testing.T) {
	_, _, addr := newTestServer(t, 10)
	conn := dial(t, addr)
	r := bufio.NewReader(conn)

	send(t, conn, "QUIT\r\n")
	if got := readReply(t, r); got != "OK" {
		t.Fatalf("QUIT = %q; want OK", got)
	}

	// The server must close the connection after +OK.
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadByte(); err == nil {
		t.Fatal("expected connection to be closed after QUIT")
	}
}

func TestServerConcurrentClients(t *testing.T) {
	_, _, addr := newTestServer(t, 1000)

	const clients = 20
	const ops = 50

	var wg sync.WaitGroup
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Errorf("client %d dial: %v", id, err)
				return
			}
			defer conn.Close()
			r := bufio.NewReader(conn)

			key := fmt.Sprintf("key-%d", id)
			for j := 0; j < ops; j++ {
				val := fmt.Sprintf("value-%d-%d", id, j)

				if _, err := conn.Write([]byte(fmt.Sprintf("SET %s %s\r\n", key, val))); err != nil {
					t.Errorf("client %d set: %v", id, err)
					return
				}
				if got, err := readReplyErr(r); err != nil || got != "OK" {
					t.Errorf("client %d SET reply = %q, %v; want OK", id, got, err)
					return
				}

				if _, err := conn.Write([]byte(fmt.Sprintf("GET %s\r\n", key))); err != nil {
					t.Errorf("client %d get: %v", id, err)
					return
				}
				if got, err := readReplyErr(r); err != nil || got != val {
					t.Errorf("client %d GET reply = %q, %v; want %q", id, got, err, val)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestServerGracefulShutdown(t *testing.T) {
	_, srv, addr := newTestServer(t, 10)

	conn := dial(t, addr)
	r := bufio.NewReader(conn)

	send(t, conn, "PING\r\n")
	if got := readReply(t, r); got != "PONG" {
		t.Fatalf("PING = %q; want PONG", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Shutdown(ctx) }()

	// The already-open connection drains: its next request still completes.
	send(t, conn, "PING\r\n")
	if got := readReply(t, r); got != "PONG" {
		t.Fatalf("drain PING = %q; want PONG", got)
	}

	// Closing the last in-flight connection lets Shutdown finish.
	_ = conn.Close()

	if err := <-done; err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// New connections must be rejected after shutdown.
	if nc, err := net.Dial("tcp", addr); err == nil {
		_ = nc.Close()
		t.Fatal("expected dial to fail after shutdown")
	}
}
