# ns8-vikunja — Vikunja 2.8.0 candidate

NS8 integration derived from [geniusdynamics/ns8-vikunja](https://github.com/geniusdynamics/ns8-vikunja).
It uses the official combined Vikunja image and PostgreSQL 15. Upstream attribution
and GPL license are retained; Vikunja itself is AGPL licensed.

**Development candidate, not yet accepted for production.** Source preparation is
local. The proposed owner repository and GHCR image require authorized publication.

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
REPOBASE=ghcr.io/amulyawan IMAGETAG=vikunja-2.8.0 bash build-images.sh
```

The `Publish images` GitHub workflow is manual. After owner authorization and a
successful registry build, the candidate image is expected at
`ghcr.io/amulyawan/vikunja:vikunja-2.8.0`. Do not assume it already exists.

## Install on an approved NS8 staging node

Only after image publication, prerequisite checks and a verified rollback source:

```bash
add-module ghcr.io/amulyawan/vikunja:vikunja-2.8.0 1
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
