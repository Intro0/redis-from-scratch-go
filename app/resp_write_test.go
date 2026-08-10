package main

import "testing"

func TestEncodeSimpleString(t *testing.T) {
	got := string(encodeSimpleString("OK"))
	want := "+OK\r\n"

	if got != want {
		t.Errorf("simple string = %q, want %q", got, want)
	}
}

func TestEncodeError(t *testing.T) {
	got := string(encodeError("ERR invalid command"))
	want := "-ERR invalid command\r\n"

	if got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestEncodeBulkString(t *testing.T) {
	got := string(encodeBulkString("Kenny"))
	want := "$5\r\nKenny\r\n"

	if got != want {
		t.Errorf("bulk string = %q, want %q", got, want)
	}
}

func TestEncodeNullBulkString(t *testing.T) {
	got := string(encodeNullBulkString())
	want := "$-1\r\n"

	if got != want {
		t.Errorf("null bulk string = %q, want %q", got, want)
	}
}

func TestEncodeInteger(t *testing.T) {
	got := string(encodeInteger(3))
	want := ":3\r\n"

	if got != want {
		t.Errorf("integer = %q, want %q", got, want)
	}
}

func TestEncodeBulkStringArray(t *testing.T) {
	got := string(encodeBulkStringArray([]string{"appendonly", "yes"}))
	want := "*2\r\n$10\r\nappendonly\r\n$3\r\nyes\r\n"

	if got != want {
		t.Errorf("bulk string array = %q, want %q", got, want)
	}
}
