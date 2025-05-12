// Copyright 2025 Vadim Filin
// SPDX-License-Identifier: gpl

//go:build !windows && !darwin

package security

import (
	"bytes"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"os"
	"strings"
)

func init() {
	_ = os.Unsetenv("LD_PRELOAD")
	//enableCoreDumps()
	disableCoreDumps()
	mustNotGDBAttached()
}

func enableCoreDumps() {
	err := unix.Prctl(unix.PR_SET_DUMPABLE, 1, 0, 0, 0)
	if err != nil {
		log.Fatalf("failed to enable core dumps: %v", err)
	}
}

func disableCoreDumps() {
	//tick := time.NewTicker(10 * time.Second)
	//defer tick.Stop()
	//<-tick.C
	err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
	if err != nil {
		log.Fatalf("failed to disable core dumps: %v", err)
	}
}

func mustNotGDBAttached() {
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
