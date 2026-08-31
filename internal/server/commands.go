package server

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// dispatch executes a single parsed command, writes its reply to w, and reports
// whether the connection should be closed (QUIT). The returned error is non-nil
// when a command-level error reply was produced, so the caller can count it
// toward the error metric; a nil error still means a valid reply was written.
func (s *Server) dispatch(w io.Writer, args []string) (closeConn bool, cmdErr error) {
	verb := strings.ToUpper(args[0])

	switch verb {
	case "PING":
		if len(args) != 1 {
			return false, s.arityError(w, verb)
		}
		return false, writeSimpleString(w, "PONG")

	case "GET":
		if len(args) != 2 {
			return false, s.arityError(w, verb)
		}
		v, ok := s.cache.Get(args[1])
		if !ok {
			return false, writeNullBulk(w)
		}
		str, ok := v.(string)
		if !ok {
			return false, s.fail(w, "value is not a string")
		}
		return false, writeBulkString(w, str)

	case "SET":
		// SET key value [EX seconds]
		if len(args) != 3 && len(args) != 5 {
			return false, s.arityError(w, verb)
		}
		ttl := time.Duration(0)
		if len(args) == 5 {
			if !strings.EqualFold(args[3], "EX") {
				return false, s.fail(w, "syntax error")
			}
			secs, err := strconv.Atoi(args[4])
			if err != nil || secs <= 0 {
				return false, s.fail(w, "invalid expire time")
			}
			ttl = time.Duration(secs) * time.Second
		}
		s.cache.Set(args[1], args[2], ttl)
		return false, writeSimpleString(w, "OK")

	case "DEL":
		if len(args) != 2 {
			return false, s.arityError(w, verb)
		}
		// Get detects presence (and handles TTL). It also promotes recency,
		// but the key is deleted immediately after, so the effect is moot.
		_, present := s.cache.Get(args[1])
		s.cache.Delete(args[1])
		if present {
			return false, writeInteger(w, 1)
		}
		return false, writeInteger(w, 0)

	case "STATS":
		if len(args) != 1 {
			return false, s.arityError(w, verb)
		}
		return false, writeBulkString(w, s.statsString())

	case "QUIT":
		if len(args) != 1 {
			return false, s.arityError(w, verb)
		}
		return true, writeSimpleString(w, "OK")

	default:
		return false, s.fail(w, "unknown command '%s'", args[0])
	}
}

// arityError writes and returns a wrong-number-of-arguments error.
func (s *Server) arityError(w io.Writer, verb string) error {
	return s.fail(w, "wrong number of arguments for '%s' command", strings.ToLower(verb))
}

// fail writes an -ERR reply and returns the underlying error.
func (s *Server) fail(w io.Writer, format string, a ...any) error {
	err := fmt.Errorf(format, a...)
	_ = writeErrorReply(w, err.Error())
	return err
}

// statsString renders the server counters for the STATS command.
func (s *Server) statsString() string {
	return fmt.Sprintf(
		"commands:%d\nactive_connections:%d\nerrors:%d",
		s.commands.Load(),
		s.activeConns.Load(),
		s.errors.Load(),
	)
}
