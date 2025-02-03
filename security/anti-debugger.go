package security

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"os"
	"strings"
)

func EnableCoreDumps() {
	err := unix.Prctl(unix.PR_SET_DUMPABLE, 1, 0, 0, 0)
	if err != nil {
		log.Fatalf("Failed to enable core dumps: %v", err)
	}
}

func DisableCoreDumps() {
	_ = os.Unsetenv("LD_PRELOAD")
	err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
	if err != nil {
		log.Fatalf("failed to disable core dumps: %v", err)
	}
}

func MustNotGDBAttached() {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return
	}
	if !bytes.Contains(data, []byte("TracerPid:\t0")) {
		output, err := os.ReadFile("/proc/self/cmdline")
		if err == nil {
			cmd := strings.ToLower(string(output))
			if strings.Contains(cmd, "gdb") || strings.Contains(cmd, "lldb") {
				panic("gdb detected")
			}
		}
	}
	return
}
