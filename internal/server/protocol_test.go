package server

import (
	"bytes"
	"slices"
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"ping", "PING\r\n", []string{"PING"}},
		{"set", "SET foo bar\r\n", []string{"SET", "foo", "bar"}},
		{"set ex", "SET foo bar EX 60\r\n", []string{"SET", "foo", "bar", "EX", "60"}},
		{"lowercase", "get foo\r\n", []string{"get", "foo"}},
		{"extra spaces", "  GET   foo  \r\n", []string{"GET", "foo"}},
		{"lf only", "PING\n", []string{"PING"}},
		{"blank crlf", "\r\n", nil},
		{"empty", "", nil},
		{"whitespace only", "   \r\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseCommand(tt.in); !slices.Equal(got, tt.want) {
				t.Fatalf("parseCommand(%q) = %#v; want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestWriters(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*bytes.Buffer) error
		want string
	}{
		{"simple", func(b *bytes.Buffer) error { return writeSimpleString(b, "OK") }, "+OK\r\n"},
		{"simple pong", func(b *bytes.Buffer) error { return writeSimpleString(b, "PONG") }, "+PONG\r\n"},
		{"error", func(b *bytes.Buffer) error { return writeErrorReply(b, "boom") }, "-ERR boom\r\n"},
		{"integer", func(b *bytes.Buffer) error { return writeInteger(b, 7) }, ":7\r\n"},
		{"integer zero", func(b *bytes.Buffer) error { return writeInteger(b, 0) }, ":0\r\n"},
		{"null bulk", func(b *bytes.Buffer) error { return writeNullBulk(b) }, "$-1\r\n"},
		{"bulk", func(b *bytes.Buffer) error { return writeBulkString(b, "hello") }, "$5\r\nhello\r\n"},
		{"bulk empty", func(b *bytes.Buffer) error { return writeBulkString(b, "") }, "$0\r\n\r\n"},
		{"bulk with newline", func(b *bytes.Buffer) error { return writeBulkString(b, "a\nb") }, "$3\r\na\nb\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b bytes.Buffer
			if err := tt.fn(&b); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := b.String(); got != tt.want {
				t.Fatalf("got %q; want %q", got, tt.want)
			}
		})
	}
}
