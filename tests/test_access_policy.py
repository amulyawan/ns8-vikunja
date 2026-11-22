import contextlib
import io
import json
import os
from pathlib import Path
import runpy
import types
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]


class NetworkAccessPolicy(unittest.TestCase):
    def configure(self, request, existing):
        calls = []
        values = {}
        agent = types.ModuleType("agent")
        tasks = types.ModuleType("agent.tasks")
        agent.tasks = tasks
        agent.set_env = lambda k, v: values.update({k: v})
        agent.dump_env = lambda: None
        agent.resolve_agent_id = lambda _: "module/traefik1"
        agent.assert_exp = lambda result: self.assertTrue(result)

        def run(**kwargs):
            calls.append(kwargs)
            return {"exit_code": 0, "output": {"ip_allowlist": existing}}

        tasks.run = run
        env = {"MODULE_ID": "vikunja1", "TCP_PORT": "20001"}
        output = io.StringIO()
        with patch.dict("sys.modules", {"agent": agent, "agent.tasks": tasks}), \
                patch.dict(os.environ, env, clear=True), \
                patch("sys.stdin", io.StringIO(json.dumps({"host": "tasks.example.org", **request}))), \
                contextlib.redirect_stdout(output):
            runpy.run_path(str(ROOT / "imageroot/actions/configure-module/20configure_traefik"), run_name="__main__")
        self.assertEqual(output.getvalue(), "")
        return next(c["data"] for c in calls if c["action"] == "set-route"), values

    def test_hostname_change_preserves_existing_private_access_when_scope_omitted(self):
        route, state = self.configure({}, ["127.0.0.1/32"])
        self.assertEqual(route["ip_allowlist"], ["127.0.0.1/32"])
        self.assertEqual(json.loads(state["TRAEFIK_IP_ALLOWLIST"]), route["ip_allowlist"])

    def test_explicit_public_scope_clears_staging_restriction(self):
        route, state = self.configure({"ip_allowlist": []}, ["127.0.0.1/32"])
        self.assertEqual(route["ip_allowlist"], [])
        self.assertEqual(state["TRAEFIK_IP_ALLOWLIST"], "[]")

    def test_private_ipv4_ipv6_are_normalized_and_deduplicated(self):
        route, _ = self.configure({"ip_allowlist": ["192.0.2.7/24", "2001:db8::1/64", "192.0.2.0/24"]}, [])
        self.assertEqual(route["ip_allowlist"], ["192.0.2.0/24", "2001:db8::/64"])

    def test_invalid_network_is_rejected_before_configuration(self):
        agent = types.ModuleType("agent")
        states = []
        agent.set_status = states.append
        output = io.StringIO()
        with patch.dict("sys.modules", {"agent": agent}), \
                patch("sys.stdin", io.StringIO('{"ip_allowlist":["invalid"]}')), \
                contextlib.redirect_stdout(output), self.assertRaises(SystemExit) as error:
            runpy.run_path(str(ROOT / "imageroot/actions/configure-module/04validate_access"), run_name="__main__")
        self.assertEqual(error.exception.code, 2)
        self.assertEqual(states, ["validation-failed"])
        self.assertEqual(json.loads(output.getvalue())[0]["field"], "ip_allowlist")
