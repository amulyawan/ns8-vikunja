#!/usr/bin/env python3
"""Fail closed before/after publishing an owner-private GHCR candidate."""
import argparse
import datetime
import json
import os
import re
import urllib.error
import urllib.request
import urllib.parse


def check(owner, allow_absent=False, package="vikunja", withdraw_public=False):
    if not re.fullmatch(r"[A-Za-z0-9-]+", owner):
        raise ValueError("Invalid GitHub owner")
    if package not in ("vikunja", "vikunja-postgres"):
        raise ValueError("Unexpected candidate package")
    encoded_package = urllib.parse.quote("ns8-vikunja-build/" + package, safe="")
    request = urllib.request.Request(
        f"https://api.github.com/users/{owner}/packages/container/{encoded_package}",
        headers={"Authorization": "Bearer " + os.environ["GH_TOKEN"],
                 "Accept": "application/vnd.github+json",
                 "X-GitHub-Api-Version": "2022-11-28"})
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            metadata = json.load(response)
            visibility = metadata["visibility"]
    except urllib.error.HTTPError as error:
        if error.code == 404 and allow_absent:
            print("Package absent: GitHub creates new container packages as private.")
            return
        raise RuntimeError(f"Cannot verify package visibility: HTTP {error.code}") from None
    if visibility != "private":
        if visibility == "public" and withdraw_public:
            # Remediate only a newly created, single-version task artifact.
            created = datetime.datetime.fromisoformat(metadata["created_at"].replace("Z", "+00:00"))
            age = (datetime.datetime.now(datetime.timezone.utc) - created).total_seconds()
            if not 0 <= age < 7200:
                raise RuntimeError("Refusing to withdraw an older package")
            versions_request = urllib.request.Request(request.full_url + "/versions", headers=dict(request.headers))
            with urllib.request.urlopen(versions_request, timeout=30) as response:
                versions = json.load(response)
            expected_tag = "vikunja-2.8.0" if package == "vikunja" else "19.0-ns8.1"
            if len(versions) != 1 or versions[0]["metadata"]["container"]["tags"] != [expected_tag]:
                raise RuntimeError("Refusing to withdraw unexpected package history")
            deletion = urllib.request.Request(request.full_url, method="DELETE", headers=dict(request.headers))
            with urllib.request.urlopen(deletion, timeout=30):
                pass
            print("Withdrawn unintended public candidate:", package)
        raise RuntimeError("Refusing candidate publication: existing package is not private")
    print("Verified GHCR package visibility: private")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("owner")
    parser.add_argument("--allow-absent", action="store_true")
    parser.add_argument("--package", default="vikunja")
    parser.add_argument("--withdraw-public", action="store_true")
    args = parser.parse_args()
    check(args.owner, args.allow_absent, args.package, args.withdraw_public)
