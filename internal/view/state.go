package view

// StateText is a participant state as icon and word, as top shows it. Its colour is the caller's.
func StateText(state string) string {
	switch state {
	case "working":
		return "● working"
	case "idle":
		return "○ idle"
	case "requested":
		return "◌ queued" // the state is `requested`; top says what it is for a person
	case "starting":
		return "◌ starting"
	case "awaiting_permission":
		return "◐ waiting"
	case "parked":
		return "⏸ parked"
	case "gone":
		return "✗ gone"
	}
	return state
}

// Status is what a state asks of the operator, for the colour of its icon: working is fine, idle is
// quiet, waiting wants a look, gone is over.
type Status int

const (
	StatusIdle    Status = iota // idle, and a state this build does not know
	StatusWorking               // working
	StatusWaiting               // queued, starting, waiting on a permission, parked
	StatusGone                  // gone
)

// Word is the status as a word, as it is encoded.
func (s Status) Word() string { return [...]string{"idle", "working", "waiting", "gone"}[s] }

func (s Status) MarshalText() ([]byte, error) { return []byte(s.Word()), nil }

// Text is the state text of the plain state of s (working, idle, awaiting_permission, gone): a column
// of that status is titled with it. A row in that plain state has exactly this state_text; a queued
// or starting one does not.
func (s Status) Text() string {
	return StateText([...]string{"idle", "working", "awaiting_permission", "gone"}[s])
}

// StatusOf is the status of a participant state.
func StatusOf(state string) Status {
	switch state {
	case "working":
		return StatusWorking
	case "requested", "starting", "awaiting_permission", "parked":
		return StatusWaiting
	case "gone":
		return StatusGone
	}
	return StatusIdle
}

// AllStates are the states a participant can be in, for the width of a STATE column (which sizes to
// the widest word StateText gives for them, `requested` being shown as `queued`).
var AllStates = []string{"requested", "starting", "working", "idle", "awaiting_permission", "parked", "gone"}

// StatusNamed is the status whose Word is w.
func StatusNamed(w string) (Status, bool) {
	for _, s := range []Status{StatusIdle, StatusWorking, StatusWaiting, StatusGone} {
		if s.Word() == w {
			return s, true
		}
	}
	return 0, false
}
