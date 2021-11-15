# kstrace

This tool is a kubectl plugin that is used to collect strace data from Pods running wiht your Kubernetes cluster.

## Description

This tool is a kubectl plugin used for watching and reviewing system-calls from processes running inside Pods in a Kubernetes cluster. 

The tool creates a privileged Pod in the cluster which will 'attach' to the running target Pod and will stream the strace data back to the end user.

This application also allows for strace monitoring of multiple Pods (DaemonSets, Deployments, Services) at the same time by streaming the results back into a designated folder.

## Installation

### Install using [Krew](https://krew.sigs.k8s.io/):
~~~
kubectl krew install strace
~~~

### Build from source:
~~~
# Build for the current OS
make build && cd bin/

# Cross-compile for all supported OSes
make all && cd bin/
~~~

## Getting Started

To start watching a Pods strace calls on the command-line:
~~~
kubectl strace -o - <pod>
~~~

Multiple Pods or containers can be traced at the same time and collected into folders. 
~~~
kubectl strace --trace-timeout=30s deployment/<deployment>
~~~

The kstrace application can trace the following Kubernetes resources identified by either their long name or short name: Deployment, DaemonSet, Service, Pod

The command flags for kstrace are listed below:
~~~
      --image string             The trace image for use when performing the strace. (default "quay.io/mwasher/crictl:0.0.2")
      --log-level string         The verbosity level of the output from the command. Available options are [panic, fatal, error, warning, info, debug, trace]. (default "info")
  -n, --namespace string         If present, the namespace scope for this CLI request
  -o, --output string            The directory to store the strace data. (default "strace-collection")
      --socket-path string       The location of the CRI socket on the host machine. (default "/run/crio/crio.sock")
      --trace-timeout string     The length of time to capture the strace output for. (default "0")
~~~

## Limitations

Auto-completion of Kubectl plugins is currently not possible but is an active development. [Kubernetes Issue 74178](https://github.com/kubernetes/kubernetes/issues/74178)

When this functionality is merged into the Kubectl tool, this tool will be updated to be compliant with the Kubectl completion mechanisms.

## Credits

This tool was inspired heavily by [Ksniff](https://github.com/eldadru/ksniff/blob/master/README.md). The ksniff tool provides a similar functionality for collecting network packet captures (PCAP) and was created by [Eldad Rudich](https://github.com/eldadru) and maintained by [Robert Bost](https://github.com/bostrt). 
Although this tool was written independently to the KSniff tool, the developer for this tool has also worked on the Ksniff tool and portions may resemble the design of KSniff.
