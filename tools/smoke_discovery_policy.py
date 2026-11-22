#!/usr/bin/env python3
"""Prove direct API clients cannot disable mandatory discovery in a test stack."""
import json
from pathlib import Path
import sys
import urllib.request

base = sys.argv[1].rstrip("/") + "/api/v2"
state = json.loads(Path(sys.argv[2]).read_text())

def call(path, data=None, token=None, method=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(base + path, headers=headers, method=method,
        data=json.dumps(data).encode() if data is not None else None)
    with urllib.request.urlopen(request, timeout=15) as response:
        return json.load(response)

token = call("/login", {"username": state["username"], "password": state["password"]})["token"]
settings = call("/user/settings/general", token=token)
assert settings["discoverable_by_name"] and settings["discoverable_by_email"]
attempt = dict(settings, discoverable_by_name=False, discoverable_by_email=False)
call("/user/settings/general", attempt, token, method="PUT")
after = call("/user/settings/general", token=token)
assert after["discoverable_by_name"] and after["discoverable_by_email"]
for key in ("name", "timezone", "week_start", "language", "default_project_id"):
    assert after.get(key) == settings.get(key), key
print("PASS: mandatory name/email discovery survives direct API disable request")
