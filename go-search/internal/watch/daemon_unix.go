//go:build !windows

package watch

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// StartDaemon starts the watcher as a background daemon
func StartDaemon(path, store, serverAddr string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}

	args := []string{"watch", path, "-s", store, "--foreground"}
	if serverAddr != "" {
		args = append(args, "-S", serverAddr)
	}

	cmd := exec.Command(exe, args...)

	// Unix-specific detachment
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	logDir := StateDir()
	os.MkdirAll(logDir, 0755)

	logFile, err := os.OpenFile(
		filepath.Join(logDir, "daemon_startup.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	go cmd.Wait()

	return cmd.Process.Pid, nil
}

// StopDaemon stops a watcher daemon by PID
func StopDaemon(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return process.Kill()
	}
	RemoveState(pid)
	return nil
}

// StopAllDaemons stops all running watcher daemons
func StopAllDaemons() (int, error) {
	states, err := ListStates()
	if err != nil {
		return 0, err
	}

	stopped := 0
	for _, state := range states {
		if err := StopDaemon(state.PID); err == nil {
			stopped++
		}
	}

	return stopped, nil
}

// isProcessRunning checks if a process is still running
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
