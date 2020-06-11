package utils

import (
	"fmt"
	"os"
	"os/exec"
)

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func PrintError(msg string, a ...interface{}) {
	fmt.Fprintln(os.Stderr, msg, a)
}

// ExecSudo is equivalent to exec.Command with sudo prefixed
func ExecSudo(arg ...string) *exec.Cmd {
	return exec.Command("sudo", arg...)
}
