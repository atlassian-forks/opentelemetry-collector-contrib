//go:build !linux

package filesystemscraper

func (cfg *Config) SetRootPath(_ string) {}
