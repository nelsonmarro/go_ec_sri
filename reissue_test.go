package sri

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Ficha técnica 2.34, §5.10: a rejected document is sent again with the same
// access key and sequential, except for these codes.
func TestReissuePlan(t *testing.T) {
	msgs := func(codes ...string) []Message {
		out := make([]Message, len(codes))
		for i, c := range codes {
			out[i] = Message{Identifier: c, Type: MessageError}
		}
		return out
	}
	cases := []struct {
		name string
		in   []Message
		want Plan
	}{
		{"no messages", nil, Plan{}},
		{"45 sequential registered", msgs("45"), Plan{NewSequential: true, NewKey: true}},
		{"58 key does not match", msgs("58"), Plan{NewKey: true}},
		{"65 sent too late", msgs("65"), Plan{NewKey: true}},
		{"67 invalid date", msgs("67"), Plan{NewKey: true}},
		{"other codes keep key and sequential", msgs("35", "39", "43", "52", "59", "70", "999"), Plan{}},
		{"the strongest wins in any order", msgs("58", "35", "45"), Plan{NewSequential: true, NewKey: true}},
		{"spaces around the code", []Message{{Identifier: " 65 "}}, Plan{NewKey: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, ReissuePlan(c.in))
		})
	}
}
