#!/usr/bin/env python3
"""Exercise the real directory HTTP boundary in the disposable local-mode stack."""
import json
from pathlib import Path
import sys
import urllib.error
import urllib.request

base = sys.argv[1].rstrip("/") + "/api/v2"
state = json.loads(Path(sys.argv[2]).read_text())

def call(path, data=None, token=None, expected=200):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(base + path, headers=headers,
        data=json.dumps(data).encode() if data is not None else None)
    try:
        with urllib.request.urlopen(request, timeout=15) as response:
            assert response.status == expected, (path, response.status)
            return json.load(response)
    except urllib.error.HTTPError as error:
        assert error.code == expected, (path, error.code)
        return None

call("/ns8/directory/status", expected=401)
token = call("/login", {"username": state["username"], "password": state["password"]})["token"]
assert call("/ns8/directory/status", token=token)["enabled"] is False
call("/ns8/directory/groups", {"password": "unused-synthetic-password"}, token, expected=403)
# LDAP-disabled instances retain normal local team creation.
call("/teams", {"name": "Synthetic local-mode team"}, token, expected=201)
print("PASS: directory namespace rejects anonymous/local writeback and retains local teams")
