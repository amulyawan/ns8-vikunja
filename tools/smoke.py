#!/usr/bin/env python3
"""Check an already-running candidate; no credentials or mutations required."""
import argparse
import json
import urllib.error
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument("base_url", help="Trusted HTTPS URL, or loopback URL on the NS8 node")
parser.add_argument("--version", default="2.8.0")
args = parser.parse_args()
base = args.base_url.rstrip("/")
for path in ("/", "/api/v2/info", "/api/v2/health"):
    with urllib.request.urlopen(base + path, timeout=15) as response:
        assert response.status == 200, (path, response.status)
        if path.endswith("/info"):
            info = json.load(response)
            assert info["version"].lstrip("v") == args.version, "Unexpected application version"
        elif path.endswith("/health"):
            assert json.load(response)["status"] == "OK", "Unhealthy dependencies"
request = urllib.request.Request(base + "/api/v2/mcp", data=b"{}",
                                 headers={"Content-Type": "application/json"})
try:
    urllib.request.urlopen(request, timeout=15)
except urllib.error.HTTPError as error:
    assert error.code in (401, 403), f"Unexpected anonymous MCP response: {error.code}"
else:
    raise AssertionError("MCP accepted an unauthenticated request")
print("PASS: frontend, API version, database health, anonymous MCP rejection")
