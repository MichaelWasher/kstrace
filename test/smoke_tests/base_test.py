import unittest
import os
import subprocess as sp
import logging
import tempfile
import string
import glob

from random import choice
from typing import Tuple


class TestE2EBase(unittest.TestCase):
    """Used as a base for E2E test Objects"""
    # TODO On test failure dump the kstrace logs

    def __init__(self, *args, **kwargs):
        super(TestE2EBase, self).__init__(*args, **kwargs)
        # Directories
        self.repo_dir = os.getcwd() + "/.."
        self.test_dir = self.repo_dir + "/test"
        self.asset_directory = self.test_dir + "/assets"
        # Binaries
        self.kstrace_bin = self.repo_dir + "/bin/kubectl-strace"
        self.socket_path = "/run/k3s/containerd/containerd.sock"

        # Defaults
        self.default_args = f"--log-file=kstrace.log --log-level=trace --socket-path={self.socket_path} --trace-timeout=10s"
        self.kubectl = "kubectl"
        self.test_assets = []

    def setUp(self):
        """Setup the test for execution"""
        self.test_namespace = self.get_test_name_processed(
        ) + "-" + "".join(choice(string.ascii_lowercase) for i in range(4))
        self.create_test_namespace()
        self.create_test_assets()

    def tearDown(self):
        """Clean up the cluster for next test (NOTE: This does not block)"""
        self.delete_test_assets()
        self.delete_test_namespace()

    def run_command(self, cmd: str) -> Tuple[int, str, str]:
        """Execute a command on the local machine"""
        proc = sp.Popen(cmd.split(" "), stdout=sp.PIPE,
                        stderr=sp.STDOUT, universal_newlines=True,)
        status = proc.wait()  # will wait for sp to finish
        out, err = proc.communicate()
        return status, out, err

    def run_kstrace(self, test_args: str) -> Tuple[int, str, str]:
        """Execute kstrace with the default and provided args"""
        return self.run_command(f"{self.kstrace_bin} -n {self.test_namespace} {self.default_args} {test_args}")

    def run_kubectl(self, kubectl_args):
        """Execute kubectl with the provided args"""
        status, _, _ = self.run_command(f"{self.kubectl} {kubectl_args}")
        return status

    def get_test_name_processed(self) -> str:
        """Get a DNS-compliant name of the test"""
        return self._testMethodName.lower().replace("_", "-").replace(" ", "")

    def create_test_namespace(self):
        """Create a namespace for the test"""
        logging.info(f"Creating Namespace {self.test_namespace}")
        self.run_kubectl(f"create ns {self.test_namespace}")

    def delete_test_namespace(self):
        """Delete the test namespace (NOTE: This does not block)"""
        logging.info(f"Deleting Namespace {self.test_namespace}")
        self.run_kubectl(
            f"delete ns --force --wait=false {self.test_namespace}")

    def create_test_assets(self):
        """Create the Kubernetes assets for the test into the test namespace"""
        if len(self.test_assets) < 1:
            return

        logging.info(f"Creating Test Assets {self.test_assets}")
        for asset in self.test_assets:
            self.run_kubectl(
                f"apply --wait -n {self.test_namespace} -f {self.asset_directory}/{asset}")
            self.wait_for_available(f"{self.asset_directory}/{asset}")

    def wait_for_available(self, asset: str):
        """Wait for the test assets to become Ready/Available"""
        commands = [
            f"{self.kubectl} wait -n {self.test_namespace} -f {asset} --for=condition=Ready=true",
            f"{self.kubectl} wait -n {self.test_namespace} -f {asset} --for=condition=Available=true",
        ]
        procs = [sp.Popen(command.split(" ")) for command in commands]

        watched_pids = set(proc.pid for proc in procs)
        while True:
            pid, _ = os.wait()
            if pid in watched_pids:
                break

        for proc in procs:
            proc.terminate()
            proc.wait()

    def delete_test_assets(self):
        """Delete the test assets from the test namespace (NOTE: This does not block)"""
        if len(self.test_assets) < 1:
            return

        logging.info(f"Deleting Test Assets {self.test_assets}")
        for asset in self.test_assets:
            self.run_kubectl(
                f"delete --force --wait=false -f {self.asset_directory}/{asset}")

    def get_logs_from_folder(self, tmp_dir) -> dict:
        """Collect the log filepaths and log contents output from the kstrace tool in a dictionary"""
        file_and_contents = {}
        for file in glob.iglob(tmp_dir + "/**/*.log", recursive=True):
            with open(file) as open_file:
                file_and_contents[file] = open_file.read()

        return file_and_contents


class PodTest(TestE2EBase):
    """A class designed for testing the kstrace tool with the pod resource type"""

    def __init__(self, *args, **kwargs):
        super(PodTest, self).__init__(*args, **kwargs)
        self.target_asset = "pod/target-pod"
        self.test_assets = ["target_pod.yaml"]

    def test_pod(self):
        # Testing the --output - option
        _, out, _ = self.run_kstrace(f"--output - {self.target_asset}")
        assert("execve" in out)


class DeploymentTest(TestE2EBase):
    """A class designed for testing the kstrace tool with the deployment resource type"""

    def __init__(self, *args, **kwargs):
        super(DeploymentTest, self).__init__(*args, **kwargs)
        self.expected_log_prefix = "target-deployment"
        self.target_asset = "deploy/target-deployment"
        self.test_assets = ["target_deployment.yaml"]

    def test_deployment(self):
        # Create the command
        with tempfile.TemporaryDirectory() as tmp_dir:
            # Testing the --output args
            status, out, err = self.run_kstrace(
                f"--output {tmp_dir} {self.target_asset}")
            log_contents = self.get_logs_from_folder(tmp_dir)

            for key in log_contents:
                assert("execve" in log_contents[key])


if __name__ == '__main__':
    logging.basicConfig(filename="./test_logs.log",
                        encoding='utf-8', level=logging.DEBUG)
    unittest.main()
