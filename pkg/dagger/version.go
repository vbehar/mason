package dagger

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func Version() (string, error) {
	var output bytes.Buffer

	cmd := exec.Command("dagger", "version")
	cmd.Env = append(cmd.Environ(), "DAGGER_NO_NAG=1")
	cmd.Stdout = &output
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to execute dagger version: %w", err)
	}

	version := output.String()
	return version, nil
}
