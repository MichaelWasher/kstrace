package main

import (
	"os"

	"github.com/michaelwasher/kube-strace/cmd"
	"github.com/spf13/pflag"
)

func main() {
	flags := pflag.NewFlagSet("kubectl-strace", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewKubeStraceCommand()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
