package utils

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
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

// GetDirectoryFromKey returns the directory name specified by the given key
// in the configuration file, and creates it if it does not exists
func GetDirectoryFromKey(key, defaultPath string) string {
	// Check if a directory is configured
	if viper.IsSet(key) {
		configDir := viper.GetString(key)

		stat, err := os.Stat(configDir)
		if err == nil {
			if !stat.IsDir() {
				Fatalf("GetDirectoryFromKey: specified path (%s) is not a directory\n", configDir)
			}
			return configDir
		}

		if os.IsNotExist(err) { // create directory if it is not present yet
			if e := os.Mkdir(configDir, os.ModeDir); e != nil {
				Fatalln("GetDirectoryFromKey: error creating directory")
			}
			return configDir
		}
		Fatalf("GetDirectoryFromKey: configured directory error: %v\n", err)
	}

	defaultDir, err := homedir.Expand(defaultPath)
	if err != nil {
		Fatalln(err)
	}

	if _, err := os.Stat(defaultDir); os.IsNotExist(err) {
		if e := os.Mkdir(defaultDir, os.ModeDir|os.ModePerm); e != nil {
			Fatalf("GetDirectoryFromKey: error creating directory at %s", defaultDir)
		}
	} else if err != nil {
		Fatalf("GetDirectoryFromKey: configured directory error: %v\n", err)
	}
	return defaultDir
}
