//go:build linux

package hostmetricsreceiver

import (
	"os"
	"path/filepath"
)

func setGopsutilsEnvVars(rootPath string) error {
	if rootPath == "" {
		return nil
	}

	for envVarName, defaultPath := range map[string]string{
		"HOST_PROC": "/proc",
		"HOST_SYS":  "/sys",
		"HOST_ETC":  "/sys",
		"HOST_VAR":  "/var",
		"HOST_RUN":  "/run",
		"HOST_DEV":  "/dev",
	} {
		// If env var is previously set, that takes precedence.
		if os.Getenv(envVarName) != "" {
			continue
		}
		err := os.Setenv(envVarName, filepath.Join(rootPath, defaultPath))
		if err != nil {
			return err
		}
	}
	return nil
}
