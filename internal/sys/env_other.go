//go:build !windows

package sys

import "os/exec"

// setTreeKill keeps the default cancellation (kill the process) outside
// Windows; WaitDelay bounds how long orphaned children may hold the pipes.
func setTreeKill(cmd *exec.Cmd) {}
