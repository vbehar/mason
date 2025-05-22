package dagger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/anchore/go-logger"
	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
)

type ExecScriptOpts struct {
	BinaryPath    string
	Logger        logger.Logger
	ScriptPath    string
	Env           []string
	Args          []string
	Stdout        io.Writer
	Stderr        io.Writer
	DisableOutput bool
}

func ExecScript(opts ExecScriptOpts) error {
	if opts.ScriptPath == "" {
		return fmt.Errorf("script path is required")
	}

	var (
		outputBuffer bytes.Buffer
		outputWriter io.Writer
	)
	if opts.DisableOutput {
		// let's just write directly to our buffer
		outputWriter = &outputBuffer
	} else {
		// we'll need to set Dagger's stderr to a fake terminal
		// so that we can capture the content and write it to our buffer
		primty, tty, err := pty.Open()
		if err != nil {
			return fmt.Errorf("failed to create a pty/tty: %w", err)
		}
		defer func() {
			err := primty.Close()
			if err != nil {
				opts.Logger.Warnf("failed to close pty: %s", err)
			}
		}()
		defer func() {
			err := tty.Close()
			if err != nil {
				opts.Logger.Warnf("failed to close tty: %s", err)
			}
		}()

		// Handle pty size.
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGWINCH)
		go func() {
			for range ch {
				if err := pty.InheritSize(os.Stderr, primty); err != nil {
					opts.Logger.Warnf("error resizing pty: %s", err)
				}
			}
		}()
		ch <- syscall.SIGWINCH
		defer func() { signal.Stop(ch); close(ch) }()

		// Redirect PTY output to both the real stderr and our buffer
		go func() {
			multiWriter := io.MultiWriter(os.Stderr, &outputBuffer)
			_, _ = io.Copy(multiWriter, primty)
		}()
		outputWriter = tty
	}

	args := make([]string, 0, len(opts.Args)+1)
	args = append(args, opts.Args...)
	args = append(args, opts.ScriptPath)

	cmd := exec.Command(opts.BinaryPath, args...)
	cmd.Env = append(cmd.Environ(), "DAGGER_NO_NAG=1")
	cmd.Env = append(cmd.Env, opts.Env...)

	cmd.Stderr = outputWriter
	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}

	runErr := cmd.Run()

	// parse/write the stderr output before handling the error
	// to make sure we don't lose it before returning
	var outputErrLogHandler sync.Once
	for _, line := range strings.Split(outputBuffer.String(), "\n") {
		line := strings.TrimSpace(line)
		line = ansi.Strip(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, " · home first · end last · ") {
			continue
		}
		_, err := opts.Stderr.Write([]byte(line + "\n"))
		if err != nil {
			outputErrLogHandler.Do(func() { // once is enough, don't spam the log
				opts.Logger.Warnf("failed to write Dagger's stderr: %s", err)
			})
		}
	}

	if runErr != nil {
		return fmt.Errorf("failed to execute dagger script %q: %w", opts.ScriptPath, runErr)
	}

	return nil
}
