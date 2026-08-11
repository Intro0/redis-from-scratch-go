package main

import (
	"math"
	"reflect"
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

func TestResolveXAddID(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		stream     Stream
		wantString string
		wantID     StreamID
	}{
		{
			name:       "explicit ID",
			input:      "10-2",
			wantString: "10-2",
			wantID:     StreamID{milliseconds: 10, sequence: 2},
		},
		{
			name:       "first zero timestamp sequence",
			input:      "0-*",
			wantString: "0-1",
			wantID:     StreamID{milliseconds: 0, sequence: 1},
		},
		{
			name:  "next sequence",
			input: "10-*",
			stream: Stream{entries: []StreamEntry{
				{id: "10-4"},
			}},
			wantString: "10-5",
			wantID:     StreamID{milliseconds: 10, sequence: 5},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotString, gotID := resolveXAddID(test.input, test.stream)

			if gotString != test.wantString {
				t.Errorf("ID string = %q, want %q", gotString, test.wantString)
			}
			if gotID != test.wantID {
				t.Errorf("ID = %#v, want %#v", gotID, test.wantID)
			}
		})
	}
}

func TestValidateXAddID(t *testing.T) {
	tests := []struct {
		name    string
		entryID string
		id      StreamID
		stream  Stream
		wantErr bool
	}{
		{name: "valid first ID", entryID: "1-0", id: StreamID{milliseconds: 1}},
		{name: "zero ID", entryID: "0-0", wantErr: true},
		{
			name:    "equal to last ID",
			entryID: "1-1",
			id:      StreamID{milliseconds: 1, sequence: 1},
			stream:  Stream{entries: []StreamEntry{{id: "1-1"}}},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateXAddID(test.entryID, test.id, test.stream)
			if (err != nil) != test.wantErr {
				t.Errorf("error = %v, want error: %t", err, test.wantErr)
			}
		})
	}
}

func TestParseStreamValues(t *testing.T) {
	got := parseStreamValues([]string{"temperature", "18", "humidity", "40"})
	want := map[string]string{
		"temperature": "18",
		"humidity":    "40",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("values = %v, want %v", got, want)
	}
}
