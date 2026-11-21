"""Run without NS8: python -m unittest discover -s tests -p 'test_*.py'."""
import contextlib
import io
import json
import os
from pathlib import Path
import runpy
import subprocess
import tempfile
import types
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]


class ModuleContract(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        previous = os.getcwd()
        os.chdir(self.directory.name)
        self.addCleanup(os.chdir, previous)
        self.smtp = dict(enabled=True, host="smtp.example.org", port=587,
                         username="example", password="synthetic-test-value",
                         encrypt_smtp="starttls", tls_verify=True)
        self.agent = types.ModuleType("agent")
        self.agent.read_envfile = self.read_env
        self.agent.write_envfile = self.write_env
        self.agent.redis_connect = lambda **kwargs: None
        self.agent.get_smarthost_settings = lambda _: self.smtp

    @staticmethod
    def read_env(path):
        return dict(line.split("=", 1) for line in Path(path).read_text().splitlines()
                    if line and not line.startswith("#"))

    @staticmethod
    def write_env(path, values):
        Path(path).write_text("".join(f"{k}={v}\n" for k, v in sorted(values.items())))

    def execute(self, relative, data="{}"):
        output = io.StringIO()
        with patch.dict("sys.modules", {"agent": self.agent}), \
                patch("sys.stdin", io.StringIO(data)), contextlib.redirect_stdout(output):
            runpy.run_path(str(ROOT / relative), run_name="__main__")
        self.assertEqual(output.getvalue(), "", "Action must not expose secrets on stdout")

    def test_new_install_uses_persistent_secret_and_shared_pod_database(self):
        self.execute("imageroot/actions/create-module/10configure_environment_vars")
        db = self.read_env("database.env")
        app = self.read_env("vikunja-db.env")
        self.assertEqual(app["VIKUNJA_DATABASE_PASSWORD"], db["POSTGRES_PASSWORD"])
        self.assertEqual(app["VIKUNJA_DATABASE_HOST"], "127.0.0.1:5432")
        self.assertGreaterEqual(len(app["VIKUNJA_SERVICE_SECRET"]), 32)
        self.assertNotIn("VIKUNJA_SERVICE_JWTSECRET", app)

    def test_upgrade_preserves_secret_password_and_original_backup(self):
        old = {"VIKUNJA_SERVICE_JWTSECRET": "synthetic-existing-secret",
               "VIKUNJA_DATABASE_PASSWORD": "synthetic-existing-db",
               "VIKUNJA_DATABASE_HOST": "postgresql-app"}
        self.write_env("vikunja-db.env", old)
        before = Path("vikunja-db.env").read_bytes()
        self.execute("imageroot/update-module.d/10migrate-config")
        after = self.read_env("vikunja-db.env")
        self.assertEqual(after["VIKUNJA_SERVICE_SECRET"], old["VIKUNJA_SERVICE_JWTSECRET"])
        self.assertEqual(after["VIKUNJA_DATABASE_PASSWORD"], old["VIKUNJA_DATABASE_PASSWORD"])
        self.assertEqual(after["VIKUNJA_DATABASE_HOST"], "127.0.0.1:5432")
        self.assertEqual(Path("vikunja-db.env.pre-2.8").read_bytes(), before)
        self.execute("imageroot/update-module.d/10migrate-config")
        self.assertEqual(self.read_env("vikunja-db.env"), after)
        self.assertEqual(Path("vikunja-db.env.pre-2.8").read_bytes(), before)

    def test_upgrade_respects_explicit_new_secret_and_custom_database_host(self):
        self.write_env("vikunja-db.env", {"VIKUNJA_SERVICE_SECRET": "synthetic-new",
            "VIKUNJA_SERVICE_JWTSECRET": "synthetic-old", "VIKUNJA_DATABASE_HOST": "db.example.org"})
        self.execute("imageroot/update-module.d/10migrate-config")
        after = self.read_env("vikunja-db.env")
        self.assertEqual(after["VIKUNJA_SERVICE_SECRET"], "synthetic-new")
        self.assertEqual(after["VIKUNJA_DATABASE_HOST"], "db.example.org")

    def test_update_unconfigured_instance_is_safe(self):
        self.execute("imageroot/update-module.d/10migrate-config")
        self.assertFalse(Path("vikunja-db.env").exists())

    def test_empty_new_secret_preserves_legacy_value(self):
        self.write_env("vikunja-db.env", {"VIKUNJA_SERVICE_SECRET": "",
            "VIKUNJA_SERVICE_JWTSECRET": "synthetic-existing"})
        self.execute("imageroot/update-module.d/10migrate-config")
        self.assertEqual(self.read_env("vikunja-db.env")["VIKUNJA_SERVICE_SECRET"],
                         "synthetic-existing")

    def test_failed_backup_prevents_configuration_mutation(self):
        self.write_env("vikunja-db.env", {"VIKUNJA_SERVICE_JWTSECRET": "synthetic-existing"})
        before = Path("vikunja-db.env").read_bytes()
        with patch("shutil.copyfileobj", side_effect=OSError("synthetic backup failure")):
            with self.assertRaises(OSError):
                self.execute("imageroot/update-module.d/10migrate-config")
        self.assertEqual(Path("vikunja-db.env").read_bytes(), before)
        self.assertFalse(Path("vikunja-db.env.pre-2.8").exists())
        self.assertEqual([p.name for p in Path('.').iterdir()], ["vikunja-db.env"])

    def test_fresh_smtp_settings_reach_vikunja(self):
        for mode, implicit_tls in (("starttls", "false"), ("tls", "true"), ("none", "false")):
            with self.subTest(mode=mode):
                self.smtp["encrypt_smtp"] = mode
                self.execute("imageroot/bin/discover-smarthost")
                env = self.read_env("smarthost.env")
                self.assertEqual(env["VIKUNJA_MAILER_FORCESSL"], implicit_tls)
                self.assertEqual(env["VIKUNJA_MAILER_SKIPTLSVERIFY"], "false")
                self.assertEqual(env["VIKUNJA_MAILER_PASSWORD"], self.smtp["password"])
                self.assertEqual(env["VIKUNJA_MAILER_ENABLED"], "true")
        self.smtp["enabled"] = False
        self.execute("imageroot/bin/discover-smarthost")
        self.assertEqual(self.read_env("smarthost.env")["VIKUNJA_MAILER_ENABLED"], "false")

    def test_backup_includes_identity_database_dump_and_attachments(self):
        includes = set((ROOT / "imageroot/etc/state-include.conf").read_text().splitlines())
        self.assertTrue({"state/database.env", "state/vikunja-db.env", "state/vikunja.env",
            "state/vikunja.pg_dump", "volumes/vikunja-app"}.issubset(includes))

    def test_container_reads_refreshed_mail_and_owns_attachment_volume(self):
        service = (ROOT / "imageroot/systemd/user/vikunja-app.service").read_text()
        self.assertIn("ExecStartPre=runagent discover-smarthost", service)
        self.assertIn("--env-file=%S/state/smarthost.env", service)
        self.assertIn("vikunja-app:/app/vikunja/files/:Z,U", service)

    def test_postgres19_service_and_restore_share_new_parent_volume(self):
        service = (ROOT / "imageroot/systemd/user/postgresql-app.service").read_text()
        restore = (ROOT / "imageroot/actions/restore-module/40restore_database").read_text()
        self.assertIn("postgres19-data:/var/lib/postgresql:Z", service)
        self.assertIn("postgres19-data:/var/lib/postgresql:Z", restore)
        # NS8 creates this variable from the runtime image basename vikunja-postgres.
        self.assertIn("${VIKUNJA_POSTGRES_IMAGE}", service)
        self.assertIn("${VIKUNJA_POSTGRES_IMAGE}", restore)
        self.assertNotIn("${POSTGRES_IMAGE}", service + restore)

    @unittest.skipUnless(os.name == "posix", "POSIX migration guard")
    def test_legacy_volume_requires_verified_migration_marker(self):
        tools = Path("fake-bin")
        tools.mkdir()
        podman = tools / "podman"
        podman.write_text('#!/bin/sh\nexit "$SYNTHETIC_VOLUME_STATUS"\n')
        podman.chmod(0o700)
        env = dict(os.environ, PATH=str(tools.resolve()) + os.pathsep + os.environ["PATH"])
        script = str(ROOT / "imageroot/update-module.d/05require-postgres19-migration")
        env["SYNTHETIC_VOLUME_STATUS"] = "1"
        self.assertEqual(subprocess.run(["bash", script], env=env, capture_output=True).returncode, 0)
        env["SYNTHETIC_VOLUME_STATUS"] = "0"
        self.assertNotEqual(subprocess.run(["bash", script], env=env, capture_output=True).returncode, 0)
        Path("postgres19-migration-complete").write_text("19\n")
        self.assertEqual(subprocess.run(["bash", script], env=env, capture_output=True).returncode, 0)


if __name__ == "__main__":
    unittest.main()
