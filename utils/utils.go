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

func PrintError(args ...interface{}) (n int, err error) {
	return fmt.Fprintln(os.Stderr, args...)
}

func Fatalln(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}

func Fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

// ExecSudo is equivalent to exec.Command with sudo prefixed
func ExecSudo(arg ...string) *exec.Cmd {
	return exec.Command("sudo", arg...)
}
