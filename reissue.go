package sri

import "strings"

// Plan says what a rejected document needs to be sent again. The rule of the
// SRI ficha técnica (version 2.34, §5.10) is the same access key and the same
// sequential; Plan marks the exceptions.
type Plan struct {
	// NewSequential: the sequential is already taken; a new number means a
	// new access key too.
	NewSequential bool
	// NewKey: the access key (or the date inside it) is the problem.
	NewKey bool
}

// ReissuePlan reads the plan from the messages of a rejection: code 45
// (sequential registered) needs a new sequential and a new key; 58 (key does
// not match), 65 (sent too late) and 67 (invalid date) need a new key; any
// other code keeps both.
func ReissuePlan(messages []Message) Plan {
	var plan Plan
	for _, m := range messages {
		switch strings.TrimSpace(m.Identifier) {
		case "45":
			plan.NewSequential, plan.NewKey = true, true
		case "58", "65", "67":
			plan.NewKey = true
		}
	}
	return plan
}
