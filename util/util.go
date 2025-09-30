package util

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// GetCurrentDirectory
// get current directory
func GetCurrentDirectory() (string, error) {
	// Method 1: Current Working Directory
	cwdDir, cwdErr := os.Getwd()

	// Method 2: Executable Directory
	exePath, exeErr := os.Executable()
	exeDir := filepath.Dir(exePath)

	// Method 3: Caller's File Directory
	_, filename, _, callerOk := runtime.Caller(0)
	callerDir := filepath.Dir(filename)

	// Choose the most appropriate method
	if cwdErr == nil && cwdDir != "" {
		return cwdDir, nil
	}

	if exeErr == nil && exeDir != "" {
		return exeDir, nil
	}

	if callerOk {
		return callerDir, nil
	}

	return "", fmt.Errorf("could not determine current directory")
}
