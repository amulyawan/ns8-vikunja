#!/usr/bin/env python3
"""Fail closed before/after publishing an owner-private GHCR candidate."""
import argparse
import json
import os
import re
import urllib.error
import urllib.request


def check(owner, allow_absent=False, package="vikunja"):
    if not re.fullmatch(r"[A-Za-z0-9-]+", owner):
        raise ValueError("Invalid GitHub owner")
    if package not in ("vikunja", "vikunja-postgres"):
        raise ValueError("Unexpected candidate package")
    request = urllib.request.Request(
        f"https://api.github.com/users/{owner}/packages/container/{package}",
        headers={"Authorization": "Bearer " + os.environ["GH_TOKEN"],
                 "Accept": "application/vnd.github+json",
                 "X-GitHub-Api-Version": "2022-11-28"})
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            visibility = json.load(response)["visibility"]
    except urllib.error.HTTPError as error:
        if error.code == 404 and allow_absent:
            print("Package absent: GitHub creates new container packages as private.")
            return
        raise RuntimeError(f"Cannot verify package visibility: HTTP {error.code}") from None
    if visibility != "private":
        raise RuntimeError("Refusing candidate publication: existing package is not private")
    print("Verified GHCR package visibility: private")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("owner")
    parser.add_argument("--allow-absent", action="store_true")
    parser.add_argument("--package", default="vikunja")
    args = parser.parse_args()
    check(args.owner, args.allow_absent, args.package)
