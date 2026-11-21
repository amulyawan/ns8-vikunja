import io
import datetime
import json
import os
import unittest
import urllib.error
from unittest.mock import patch

from tools.registry_visibility import check


class RegistryPolicy(unittest.TestCase):
    def invoke(self, visibility, allow_absent=False):
        response = io.BytesIO(('{"visibility":"' + visibility + '"}').encode())
        with patch.dict(os.environ, {"GH_TOKEN": "synthetic-test-token"}), \
                patch("urllib.request.urlopen", return_value=response):
            check("example-owner", allow_absent)

    def test_private_package_is_allowed(self):
        self.invoke("private")

    def test_public_package_is_rejected_even_when_creation_is_allowed(self):
        with self.assertRaisesRegex(RuntimeError, "not private"):
            self.invoke("public", allow_absent=True)

    def test_only_prepublication_may_accept_missing_package(self):
        error = urllib.error.HTTPError("https://api.github.com", 404, "Not found", {}, None)
        with patch.dict(os.environ, {"GH_TOKEN": "synthetic-test-token"}), \
                patch("urllib.request.urlopen", side_effect=error):
            check("example-owner", allow_absent=True)
            with self.assertRaisesRegex(RuntimeError, "HTTP 404"):
                check("example-owner")

    def test_permission_failure_is_rejected(self):
        error = urllib.error.HTTPError("https://api.github.com", 403, "Forbidden", {}, None)
        with patch.dict(os.environ, {"GH_TOKEN": "synthetic-test-token"}), \
                patch("urllib.request.urlopen", side_effect=error):
            with self.assertRaisesRegex(RuntimeError, "HTTP 403"):
                check("example-owner", allow_absent=True)

    def test_new_public_candidate_is_withdrawn_then_run_still_fails(self):
        metadata = {"visibility": "public", "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat()}
        versions = [{"metadata": {"container": {"tags": ["vikunja-2.8.0"]}}}]
        responses = [io.BytesIO(json.dumps(metadata).encode()),
                     io.BytesIO(json.dumps(versions).encode()), io.BytesIO(b"")]
        with patch.dict(os.environ, {"GH_TOKEN": "synthetic-test-token"}), \
                patch("urllib.request.urlopen", side_effect=responses) as opener:
            with self.assertRaisesRegex(RuntimeError, "not private"):
                check("example-owner", withdraw_public=True)
            self.assertEqual(opener.call_args_list[-1].args[0].get_method(), "DELETE")

    def test_privacy_remediation_never_deletes_an_older_package(self):
        metadata = {"visibility": "public", "created_at": "2000-01-01T00:00:00Z"}
        with patch.dict(os.environ, {"GH_TOKEN": "synthetic-test-token"}), \
                patch("urllib.request.urlopen", return_value=io.BytesIO(json.dumps(metadata).encode())) as opener:
            with self.assertRaisesRegex(RuntimeError, "older package"):
                check("example-owner", withdraw_public=True)
            self.assertEqual(opener.call_count, 1)
