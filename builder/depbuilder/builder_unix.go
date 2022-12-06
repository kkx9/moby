//go:build !windows
// +build !windows

package depbuilder // import "github.com/docker/docker/builder/depbuilder"

func defaultShellForOS(os string) []string {
	return []string{"/bin/sh", "-c"}
}
