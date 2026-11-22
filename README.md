# ns8-vikunja — Vikunja 2.8.0 candidate

NS8 integration derived from [geniusdynamics/ns8-vikunja](https://github.com/geniusdynamics/ns8-vikunja).
It uses the official combined Vikunja 2.8.0 image and a patched PostgreSQL 19.0 runtime.
Fresh installs target the latest stable database, verified with a disposable application stack.
Upstream attribution
and GPL license are retained; Vikunja itself is AGPL licensed.

**Development candidate, not yet accepted for production.** Public development
source is in `amulyawan/ns8-vikunja`. Authorized owner-only publication runs from
the private `amulyawan/ns8-vikunja-build` repository, with explicit privacy gates.

- [Panduan lengkap bahasa Indonesia: fork, build, staging, upgrade, backup dan rollback](docs/PANDUAN-NS8-ID.md)
- [Validation gates and untested items](docs/VALIDATION.md)
- [Dependencies, licenses, source and observed image digests](docs/DEPENDENCIES.md)

## Validate source

Run in Linux or WSL, from this repository:

```bash
python3 tools/validate.py
```

## Build a candidate

On a Linux development host with Buildah/Podman:

```bash
REPOBASE=ghcr.io/amulyawan/ns8-vikunja-build IMAGETAG=2.8.0-rc.2 bash build-images.sh
```

The manual `Publish images` workflow completed successfully in private run
44963956861. The owner-only candidate is published at
`ghcr.io/amulyawan/ns8-vikunja-build/vikunja:2.8.0-rc.2` with the private
database runtime `ghcr.io/amulyawan/ns8-vikunja-build/vikunja-postgres:19.0-ns8.1`.
Both packages passed the before/after privacy checks. Current RC2 build/scan run: [44963956861](https://github.com/amulyawan/ns8-vikunja-build/actions/runs/44963956861).
Fresh NS8 installation, scoped MCP, attachment/restart persistence, native backup/restore and Software Center RC1→RC2 update passed on NS8 3.25.1.
See the [separate installation guide](docs/INSTALASI-UJI-NS8-ID.md) for the catalog URL and private-registry authentication. Legacy data migration and production TLS/mail remain untested.

## Install on an approved NS8 staging node

Only after image publication, prerequisite checks and a verified rollback source:

```bash
add-module ghcr.io/amulyawan/ns8-vikunja-build/vikunja:2.8.0-rc.2 1
```

Use the returned module ID, actual node ID, DNS and trusted TLS certificate.
See the Indonesian runbook before configuring or upgrading an existing instance.
After configuration, from a trusted workstation:

```bash
python3 tools/smoke.py https://vikunja.example.org
```

MCP is built into Vikunja at `/api/v2/mcp`; create a scoped token under Settings → MCP.
This smoke test checks anonymous rejection, not authenticated MCP functionality.
Never downgrade an already migrated database by changing only the image tag.
