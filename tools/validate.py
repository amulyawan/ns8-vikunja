#!/usr/bin/env python3
"""Dependency-free source checks; deliberately does not claim NS8 acceptance."""
import ast
import json
from pathlib import Path
import shutil
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
bash = shutil.which("bash")
if not bash:
    sys.exit("Bash required: run this check in WSL or a Linux development host.")
count = {"python": 0, "bash": 0, "json": 0}
paths = [root / "build-images.sh"]
for directory in ("imageroot", "tests", "tools"):
    paths.extend(p for p in (root / directory).rglob("*") if p.is_file()
                 and "__pycache__" not in p.parts)
for path in paths:
    source = path.read_text(encoding="utf-8")
    if path.suffix == ".py" or source.startswith("#!/usr/bin/env python3"):
        ast.parse(source, filename=str(path))
        count["python"] += 1
    elif source.startswith("#!/bin/bash"):
        subprocess.run([bash, "-n", str(path)], check=True)
        count["bash"] += 1
    elif path.suffix == ".json":
        json.loads(source)
        count["json"] += 1
subprocess.run([sys.executable, "-m", "unittest", "discover", "-s", "tests",
                "-p", "test_*.py", "-v"], cwd=root, check=True)
subprocess.run(["git", "diff", "--check"], cwd=root, check=True)
print(json.dumps({"source_checks": count, "ns8_runtime": "not-tested"}))
