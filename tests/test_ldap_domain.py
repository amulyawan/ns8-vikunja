"""Authentication boundaries for a native NS8 directory-backed instance."""
import contextlib
import io
import os
from pathlib import Path
import runpy
import tempfile
import types
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]


class DirectoryAuthentication(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        previous = os.getcwd()
        os.chdir(directory.name)
        self.addCleanup(os.chdir, previous)
        self.domain = {"host": "127.0.0.1", "port": "20000", "schema": "ad",
                       "base_dn": "DC=example,DC=org", "bind_dn": "reader@example.org",
                       "bind_password": "synthetic-directory-secret"}
        owner = self

        class Proxy:
            def get_domain(self, name):
                return owner.domain if name == "ad.example.org" else None

            def get_ldap_users_search_filter_clause(self, name):
                return "(!(sAMAccountName=ldapservice))"

        self.agent = types.ModuleType("agent")
        self.agent.write_envfile = lambda p, v: Path(p).write_text(
            "".join(f"{k}={value}\n" for k, value in v.items()))
        self.proxy = types.ModuleType("agent.ldapproxy")
        self.proxy.Ldapproxy = Proxy

    def discover(self, domain):
        output = io.StringIO()
        with patch.dict("sys.modules", {"agent": self.agent, "agent.ldapproxy": self.proxy}), \
                patch.dict(os.environ, {"LDAP_USER_DOMAIN": domain}), contextlib.redirect_stdout(output):
            runpy.run_path(str(ROOT / "imageroot/bin/discover-user-domain"), run_name="__main__")
        self.assertEqual(output.getvalue(), "")
        return dict(line.split("=", 1) for line in Path("ldap.env").read_text().splitlines())

    def test_domain_disables_local_login_registration_and_hidden_disabled_accounts(self):
        config = self.discover("ad.example.org")
        self.assertEqual(config["VIKUNJA_AUTH_LDAP_ENABLED"], "true")
        self.assertEqual(config["VIKUNJA_AUTH_LOCAL_ENABLED"], "false")
        self.assertEqual(config["VIKUNJA_SERVICE_ENABLEREGISTRATION"], "false")
        self.assertEqual(config["VIKUNJA_AUTH_LDAP_HOST"], "10.0.2.2")
        self.assertIn("(!(sAMAccountName=ldapservice))", config["VIKUNJA_AUTH_LDAP_USERFILTER"])
        self.assertIn("userAccountControl:1.2.840.113556.1.4.803:=2", config["VIKUNJA_AUTH_LDAP_USERFILTER"])
        self.assertEqual(config["VIKUNJA_AUTH_LDAP_ATTRIBUTE_EMAIL"], "mail")
        self.assertEqual(config["VIKUNJA_AUTH_LDAP_BINDPASSWORD"], self.domain["bind_password"])
        if os.name != "nt":
            self.assertEqual(Path("ldap.env").stat().st_mode & 0o777, 0o600)

    def test_missing_domain_fails_without_replacing_last_known_auth(self):
        self.discover("ad.example.org")
        before = Path("ldap.env").read_bytes()
        with self.assertRaises(RuntimeError):
            self.discover("missing.example.org")
        self.assertEqual(Path("ldap.env").read_bytes(), before)

    def test_bind_password_rotation_is_refreshed_on_next_start(self):
        self.discover("ad.example.org")
        self.domain["bind_password"] = "synthetic-rotated-value"
        self.assertEqual(self.discover("ad.example.org")["VIKUNJA_AUTH_LDAP_BINDPASSWORD"],
                         "synthetic-rotated-value")

    def test_no_domain_keeps_existing_local_auth_defaults(self):
        self.assertEqual(self.discover(""), {"VIKUNJA_AUTH_LDAP_ENABLED": "false"})

    def test_upn_email_mapping_and_stable_ad_identity(self):
        with patch.dict(os.environ, {"LDAP_EMAIL_ATTRIBUTE": "userPrincipalName"}):
            config = self.discover("ad.example.org")
        self.assertEqual(config["VIKUNJA_AUTH_LDAP_ATTRIBUTE_EMAIL"], "userPrincipalName")
        self.assertEqual(config["VIKUNJA_AUTH_LDAP_ATTRIBUTE_ID"], "objectGUID")
        self.assertIn("(userPrincipalName=%[1]s)", config["VIKUNJA_AUTH_LDAP_USERFILTER"])

    def test_runtime_can_reach_native_proxy_and_loads_generated_auth_last(self):
        pod = (ROOT / "imageroot/systemd/user/vikunja.service").read_text()
        app = (ROOT / "imageroot/systemd/user/vikunja-app.service").read_text()
        self.assertIn("--network=slirp4netns:allow_host_loopback=true", pod)
        self.assertLess(app.index("ExecStartPre=runagent discover-user-domain"), app.index("ExecStart="))
        self.assertLess(app.index("--env-file=%S/state/vikunja.env"), app.index("--env-file=%S/state/ldap.env"))
