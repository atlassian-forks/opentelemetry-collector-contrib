//go:build !linux

package hostmetricsreceiver

func setGopsutilsEnvVars(rootPath string) error {
	return nil
}

func setRootPath(cfg Config, rootPath string) {}
