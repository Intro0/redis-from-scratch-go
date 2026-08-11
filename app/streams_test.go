package main

import (
	"math"
	"testing"
)

func TestParseXAddID(t *testing.T) {
	tests := []struct {
		name                 string
		input                string
		want                 StreamID
		wantSequenceWildcard bool
		wantMillisecondsWild bool
	}{
		{
			name:                 "fully generated ID",
			input:                "*",
			want:                 StreamID{},
			wantSequenceWildcard: true,
			wantMillisecondsWild: true,
		},
		{
			name:                 "generated sequence",
			input:                "123-*",
			want:                 StreamID{milliseconds: 123},
			wantSequenceWildcard: true,
		},
		{
			name:  "explicit ID",
			input: "123-4",
			want:  StreamID{milliseconds: 123, sequence: 4},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, sequenceWildcard, millisecondsWildcard := parseXAddID(test.input)

			if got != test.want {
				t.Errorf("ID = %#v, want %#v", got, test.want)
			}
			if sequenceWildcard != test.wantSequenceWildcard {
				t.Errorf("sequence wildcard = %t, want %t", sequenceWildcard, test.wantSequenceWildcard)
			}
			if millisecondsWildcard != test.wantMillisecondsWild {
				t.Errorf("milliseconds wildcard = %t, want %t", millisecondsWildcard, test.wantMillisecondsWild)
			}
		})
	}
}

func TestParseRangeID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		isEnd bool
		want  StreamID
	}{
		{name: "start of stream", input: "-", want: StreamID{}},
		{name: "end of stream", input: "+", want: StreamID{milliseconds: math.MaxInt, sequence: math.MaxInt}},
		{name: "explicit ID", input: "123-4", want: StreamID{milliseconds: 123, sequence: 4}},
		{name: "start timestamp", input: "123", want: StreamID{milliseconds: 123}},
		{name: "end timestamp", input: "123", isEnd: true, want: StreamID{milliseconds: 123, sequence: math.MaxInt}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseRangeID(test.input, test.isEnd); got != test.want {
				t.Errorf("ID = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestStreamIDCompare(t *testing.T) {
	tests := []struct {
		name  string
		left  StreamID
		right StreamID
		want  int
	}{
		{name: "older timestamp", left: StreamID{milliseconds: 1}, right: StreamID{milliseconds: 2}, want: -1},
		{name: "lower sequence", left: StreamID{milliseconds: 1, sequence: 1}, right: StreamID{milliseconds: 1, sequence: 2}, want: -1},
		{name: "equal IDs", left: StreamID{milliseconds: 1, sequence: 1}, right: StreamID{milliseconds: 1, sequence: 1}, want: 0},
		{name: "newer timestamp", left: StreamID{milliseconds: 2}, right: StreamID{milliseconds: 1, sequence: 99}, want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.left.compare(test.right); got != test.want {
				t.Errorf("compare = %d, want %d", got, test.want)
			}
		})
	}
}
