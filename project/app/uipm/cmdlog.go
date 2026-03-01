package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
)

type cmdError struct {
	Message string `json:"message"`
}

var (
	cmdErrsMu sync.Mutex
	cmdErrs   []cmdError
)

func pushCmdError(msg string) {
	cmdErrsMu.Lock()
	cmdErrs = append(cmdErrs, cmdError{Message: msg})
	cmdErrsMu.Unlock()
}

func drainCmdErrors() []cmdError {
	cmdErrsMu.Lock()
	defer cmdErrsMu.Unlock()
	out := cmdErrs
	cmdErrs = nil
	return out
}

// redactArgs masks sensitive flag values before logging.
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if strings.HasPrefix(a, "--authkey=") {
			out[i] = "--authkey=***"
		} else {
			out[i] = a
		}
	}
	return out
}

// runCmd logs and runs a command, returning an error if it fails.
// Stderr/stdout are captured and printed on failure.
func runCmd(name string, args ...string) error {
	logLine := strings.Join(redactArgs(append([]string{name}, args...)), " ")
	log.Printf("[exec] $ %s", logLine)
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		var msg string
		if len(out) > 0 {
			msg = fmt.Sprintf("%s: %v — %s", name, err, strings.TrimSpace(string(out)))
		} else {
			msg = fmt.Sprintf("%s: %v", name, err)
		}
		log.Printf("[exec] error: %s", msg)
		pushCmdError(msg)
	} else {
		log.Printf("[exec] ok")
	}
	return err
}

// runCmdOutput logs and runs a command, returning stdout.
func runCmdOutput(name string, args ...string) ([]byte, error) {
	logLine := strings.Join(redactArgs(append([]string{name}, args...)), " ")
	log.Printf("[exec] $ %s", logLine)
	data, err := exec.Command(name, args...).Output()
	if err != nil {
		msg := fmt.Sprintf("%s: %v", name, err)
		log.Printf("[exec] error: %v", err)
		pushCmdError(msg)
	} else {
		log.Printf("[exec] ok (%d bytes)", len(data))
	}
	return data, err
}
