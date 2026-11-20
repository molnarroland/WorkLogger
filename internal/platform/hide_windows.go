package platform

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps a console window from flashing up for every git call.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
