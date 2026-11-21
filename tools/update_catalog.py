#!/usr/bin/env python3
"""Build a sanitized NS8 Software Center index from inspected image labels."""
import argparse
import json
from pathlib import Path
import re
import struct

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser()
parser.add_argument("version")
parser.add_argument("labels_file", type=Path)
args = parser.parse_args()
if not re.fullmatch(r"\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?", args.version):
    raise SystemExit("NS8 requires a semantic version tag")
labels = json.loads(args.labels_file.read_text())
assert labels["org.nethserver.rootfull"] == "0"
assert labels["org.nethserver.min-core"] == "3.0.0"
assert "vikunja-postgres:19.0-ns8.1" in labels["org.nethserver.images"]
assert "docker.io/vikunja/vikunja:2.8.0" in labels["org.nethserver.images"]
metadata = json.loads((root / "ui/public/metadata.json").read_text())
metadata.update(id="vikunja", name="Vikunja (amulyawan)",
    upstream_name="Vikunja 2.8.0 / PostgreSQL 19.0", logo="vikunja/logo.png", screenshots=[])
metadata["versions"] = [{"tag": args.version, "testing": "-" in args.version,
                         "labels": {k: v for k, v in labels.items() if k.startswith("org.nethserver.")}}]
destination = root / "catalog"
(destination / "vikunja").mkdir(parents=True, exist_ok=True)
logo = destination / "vikunja/logo.png"
assert struct.unpack(">II", logo.read_bytes()[16:24]) == (256, 256), "Catalog requires a 256px logo"
(destination / "repodata.json").write_text(json.dumps([metadata], indent=2) + "\n", encoding="utf-8")
print("Generated NS8 catalog from verified image labels:", args.version)
