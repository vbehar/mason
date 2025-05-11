package dagger

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

type ExecScriptOpts struct {
	ScriptPath string
	Env        []string
	Args       []string
	Stdout     io.Writer
	Stderr     io.Writer
}

func ExecScript(opts ExecScriptOpts) error {
	if opts.ScriptPath == "" {
		return fmt.Errorf("script path is required")
	}

	args := make([]string, 0, len(opts.Args)+1)
	args = append(args, opts.Args...)
	args = append(args, opts.ScriptPath)

	cmd := exec.Command("dagger", args...)
	cmd.Env = append(cmd.Environ(), "DAGGER_NO_NAG=1")
	cmd.Env = append(cmd.Env, opts.Env...)
	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute dagger script %q: %w", opts.ScriptPath, err)
	}
	return nil
}
