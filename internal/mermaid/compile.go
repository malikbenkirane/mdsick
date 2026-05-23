package mermaid

import (
	"io"
	"os/exec"
)

func Compile(src io.Reader, dst io.Writer) error {
	cmd := exec.Command("mmdc", "-i", "-", "-o", "-")
	cmd.Stdin = src
	cmd.Stdout = dst
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
