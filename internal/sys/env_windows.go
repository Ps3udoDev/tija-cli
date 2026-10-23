//go:build windows

package sys

import (
	"os/exec"
	"strconv"
)

// setTreeKill makes a cancelled cmd kill its whole process tree. Killing
// only the direct child would leave grandchildren (a .cmd shim's node, a
// script's test runner) running and holding the output pipes.
func setTreeKill(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
