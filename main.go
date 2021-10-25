package main

import (
	log "github.com/sirupsen/logrus"

	"os"

	"github.com/michaelwasher/kube-strace/cmd"
	"github.com/spf13/pflag"
)

func main() {
	flags := pflag.NewFlagSet("kubectl-strace", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewKubeStraceCommand()
	if err := root.Execute(); err != nil {
		log.Debug("The program has failed with error: %v", err)
		log.Fatal("The program has encountered a problem and needs to close. Please try again.")
		os.Exit(1)
	}
}
