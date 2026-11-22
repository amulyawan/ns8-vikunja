import contextlib
import io
import json
import os
from pathlib import Path
import runpy
import subprocess
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = Path(__file__).resolve().parents[1] / "imageroot/bin/enforce-directory-users"


class DirectoryOnlyPolicy(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        previous = os.getcwd()
        os.chdir(directory.name)
        self.addCleanup(os.chdir, previous)

    def execute(self, run, domain="ad.example.org"):
        output = io.StringIO()
        with patch.dict(os.environ, {"LDAP_USER_DOMAIN": domain}), \
                patch("subprocess.run", side_effect=run), contextlib.redirect_stdout(output):
            runpy.run_path(str(SCRIPT), run_name="__main__")
        self.assertEqual(output.getvalue(), "")

    def test_local_mode_does_not_change_account_status(self):
        with self.assertRaises(SystemExit) as error:
            self.execute(lambda *args, **kwargs: self.fail("Local mode must not invoke CLI"), domain="")
        self.assertEqual(error.exception.code, 0)
        self.assertFalse(Path("local-users-before-directory.json").exists())

    def test_backup_precedes_cli_disable_and_query_excludes_other_auth_sources(self):
        calls = []

        def run(command, **kwargs):
            calls.append(command)
            if "psql" in command:
                self.assertIn("issuer='local' AND status=0", command[-1])
                return subprocess.CompletedProcess(command, 0, '[{"id":1,"username":"synthetic","status":0}]')
            self.assertEqual(json.loads(Path("local-users-before-directory.json").read_text())["1"]["status"], 0)
            self.assertEqual(command[-3:], ["change-status", "1", "--disable"])
            return subprocess.CompletedProcess(command, 0, "")

        self.execute(run)
        self.assertEqual(len(calls), 2)

    def test_failed_disable_keeps_recovery_metadata(self):
        def run(command, **kwargs):
            if "psql" in command:
                return subprocess.CompletedProcess(command, 0, '[{"id":2,"username":"synthetic","status":0}]')
            raise subprocess.CalledProcessError(1, command)

        with self.assertRaises(subprocess.CalledProcessError):
            self.execute(run)
        self.assertTrue(Path("local-users-before-directory.json").exists())
