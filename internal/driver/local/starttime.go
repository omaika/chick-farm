package local

import "github.com/sting8k/piggery/internal/platform"

// ProcessStartTime is the OS start time of pid in unix ms, or 0 when unknown. Recovery
// compares it with the live process before killing anything, so a reused pid
// is never mistaken for the worker; a Claude session's host key uses it the same way.
func ProcessStartTime(pid int) int64 { return platform.StartTime(pid) }

// ParentPID is pid's parent, or 0 when unknown (the process is gone).
func ParentPID(pid int) int { return platform.ParentPID(pid) }
