#!/bin/bash
# basic_test.sh

ROOT_DIR=$(pwd)/..
ASSET_DIR=$ROOT_DIR/test/assets
KSTRACE_BIN=$ROOT_DIR/bin/kubectl-strace
POD_NAME=target-pod
LOG_FILE=/tmp/test_logfile.log

set -x

# Trap signals and process
trap 'trap - SIGTERM && kill 0' SIGINT SIGTERM

# Remove log file
rm -f $LOG_FILE

# Apply the target pod
kubectl apply -f $ASSET_DIR/target_pod.yaml
kubectl wait --for=condition=Ready=true pod/$POD_NAME

# Start Trace
$KSTRACE_BIN --trace-timeout=10s --log-level=trace --output - --socket-path="/run/k3s/containerd/containerd.sock" pod/$POD_NAME >/tmp/test_logfile.log &

# Check deployment with test
DEPLOYMENT_NAME=target-deployment
kubectl apply -f $ASSET_DIR/target_deployment.yaml
kubectl wait --for=condition=Available=true deploy/$DEPLOYMENT_NAME

# Start Trace
$KSTRACE_BIN --log-level=trace --socket-path="/run/k3s/containerd/containerd.sock" deploy/$DEPLOYMENT_NAME >/tmp/test_logfile.log

# Search for expected results
cat /tmp/test_logfile.log | grep execve

retVal=$?
if [ $retVal -ne 0 ]; then
    echo "The test has failed" 
    cat /tmp/test_logfile.log
    exit $retVal
else
    echo "The test has passed"
    exit 0
fi

