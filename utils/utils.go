package utils

import (
	"fmt"
	"os"
)

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func PrintError(msg string, a ...interface{}) {
	fmt.Fprintln(os.Stderr, msg, a)
}
