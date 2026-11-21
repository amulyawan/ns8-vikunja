#!/usr/bin/env python3
"""Synthetic application CRUD/persistence test against a disposable CI stack."""
import argparse
import json
from pathlib import Path
import secrets
import time
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument("base_url")
parser.add_argument("phase", choices=("create", "verify"))
parser.add_argument("state_file", type=Path)
args = parser.parse_args()
base = args.base_url.rstrip("/") + "/api/v2"


def call(path, method="GET", data=None, token=None, timeout=15):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(base + path, method=method, headers=headers,
        data=json.dumps(data).encode() if data is not None else None)
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.load(response)


deadline = time.monotonic() + 90
last_error = None
while time.monotonic() < deadline:
    try:
        if call("/health", timeout=2)["status"] == "OK":
            break
    except Exception as error:
        last_error = str(error)
        time.sleep(1)
else:
    raise RuntimeError("Vikunja/PostgreSQL stack readiness timed out: " + str(last_error))

if args.phase == "create":
    state = {"username": "ns8smoke", "password": secrets.token_urlsafe(24)}
    call("/register", "POST", {**state, "email": "smoke@example.org"})
else:
    state = json.loads(args.state_file.read_text())
token = call("/login", "POST", {"username": state["username"], "password": state["password"]})["token"]
if args.phase == "create":
    project = call("/projects", "POST", {"title": "Synthetic NS8 project"}, token)
    task = call(f"/projects/{project['id']}/tasks", "POST", {"title": "Synthetic PostgreSQL 19 task"}, token)
    state["task_id"] = task["id"]
    args.state_file.write_text(json.dumps(state))
    args.state_file.chmod(0o600)
else:
    task = call(f"/tasks/{state['task_id']}", token=token)
    assert task["title"] == "Synthetic PostgreSQL 19 task"
print("PASS: synthetic Vikunja/PostgreSQL 19 " + args.phase)
