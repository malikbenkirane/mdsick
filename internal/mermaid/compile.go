package mermaid

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	_ "embed"
)

//go:embed mmdc-config.json
var config []byte

func tempConfig() (path string, err error) {

	f, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		return "", fmt.Errorf("os: create temp: %w", err)
	}

	defer func() {
		err = errors.Join(err, f.Close())
	}()

	if _, err = f.Write(config); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}

	return f.Name(), nil

}

func Compile(src io.Reader, dst io.Writer) (err error) {

	config, err := tempConfig()
	if err != nil {
		return fmt.Errorf("prepare config file: %w", err)
	}

	defer func() {
		err = errors.Join(err, os.Remove(config))
	}()

	args := []string{
		"-c", config,
		"-b", "#ffffff",
	}

	cmd := exec.Command("mmdc", append(args, "-i", "-", "-o", "-")...)
	cmd.Stdin = src
	cmd.Stdout = dst

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil

}
