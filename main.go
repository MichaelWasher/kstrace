package main

import (
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"os"

	"github.com/michaelwasher/kube-strace/cmd"
	"github.com/spf13/pflag"
)

func main() {
	// If deployed as `kubectl-*` rename to `kubectl *` for Krew best-practices
	applicationName := filepath.Base(os.Args[0])
	if strings.HasPrefix(applicationName, "kubectl-") {
		applicationName = "kubectl strace"
	}

	flags := pflag.NewFlagSet(applicationName, pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewKubeStraceCommand(applicationName)
	if err := root.Execute(); err != nil {
		log.Debugf("%s has failed with error: %v", applicationName, err)
		log.Fatalf("%s has encountered a problem and needs to close. Please try again.", applicationName)
		os.Exit(1)
	}
}
