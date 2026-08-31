package server

import (
	"fmt"
	"io"
	"strings"
)

// parseCommand splits a raw request line into a verb and its arguments.
//
// The Phase 2 wire format is line-based inline commands, CRLF-terminated
// (e.g. "GET key\r\n"). Values cannot contain spaces yet; strings.Fields
// splits on any whitespace and strips the trailing CR/LF.
func parseCommand(line string) []string {
	line = strings.TrimRight(line, "\r\n")
	return strings.Fields(line)
}

// The response side is a small RESP-flavored subset. Each writer emits the
// exact bytes for one reply type; callers are responsible for flushing any
// buffered writer.

func writeSimpleString(w io.Writer, s string) error {
	_, err := fmt.Fprintf(w, "+%s\r\n", s)
	return err
}

func writeErrorReply(w io.Writer, msg string) error {
	_, err := fmt.Fprintf(w, "-ERR %s\r\n", msg)
	return err
}

func writeInteger(w io.Writer, n int) error {
	_, err := fmt.Fprintf(w, ":%d\r\n", n)
	return err
}

func writeNullBulk(w io.Writer) error {
	_, err := io.WriteString(w, "$-1\r\n")
	return err
}

func writeBulkString(w io.Writer, s string) error {
	if _, err := fmt.Fprintf(w, "$%d\r\n", len(s)); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "%s\r\n", s)
	return err
}
