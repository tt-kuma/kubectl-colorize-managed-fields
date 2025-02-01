package main

import (
	"os"

	"github.com/tt-kuma/kubectl-colorize-managed-fields/pkg/cmd"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func main() {
	cmd := cmd.NewCmdColorizeManagedFields(genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	})
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
