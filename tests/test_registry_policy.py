import io
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
