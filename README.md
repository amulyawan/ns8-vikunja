# ns8-vikunja — Vikunja 2.8.0 candidate

NS8 integration derived from [geniusdynamics/ns8-vikunja](https://github.com/geniusdynamics/ns8-vikunja).
It uses the official Vikunja 2.8.0 container base, a source-built executable with mandatory name/full-email discovery, and patched PostgreSQL 19.0.
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
REPOBASE=ghcr.io/amulyawan/ns8-vikunja-build IMAGETAG=2.8.0-rc.5 bash build-images.sh
```

The manual `Publish images` workflow completed successfully in private run
44975299796. The owner-only candidate is published at
`ghcr.io/amulyawan/ns8-vikunja-build/vikunja:2.8.0-rc.5` with the private
database runtime `ghcr.io/amulyawan/ns8-vikunja-build/vikunja-postgres:19.0-ns8.1`.
Both packages passed the before/after privacy checks. Current RC5 build/scan run: [44975299796](https://github.com/amulyawan/ns8-vikunja-build/actions/runs/44975299796).
Fresh NS8 installation, scoped MCP, attachment/restart persistence, native backup/restore and Software Center RC1→RC2 update passed on NS8 3.25.1.
See the [separate installation guide](docs/INSTALASI-UJI-NS8-ID.md) for the catalog URL and private-registry authentication. Trusted HTTPS and owner AD login were verified. Legacy data migration and actual SMTP delivery remain untested.

## Install on an approved NS8 staging node

Only after image publication, prerequisite checks and a verified rollback source:

```bash
add-module ghcr.io/amulyawan/ns8-vikunja-build/vikunja:2.8.0-rc.5 1
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


Network access (specific IPv4/IPv6 CIDRs or all IPs) and native NS8 LDAP user-domain
configuration are available in Settings. Directory-only mode disables prior local
accounts through the supported CLI with recovery metadata, preserving their data.
See the [LDAP/access/account guide](docs/LDAP-DAN-AKSES-NS8-ID.md).

RC5 provisions LDAP profiles before first login and maps primary/nested directory groups
to external teams on a fifteen-minute timer. Both discovery Privacy controls are
mandatory in UI and API. Directory-membership writeback from the UI is a separate
follow-up; RC5 directory access is read-only.

The RC6 candidate adds actor-authorized directory membership changes from Teams,
with fresh LDAP credential confirmation, recovery records, and independent
project management. See the [AD group administration runbook](docs/ADMIN-GROUP-AD-ID.md)
for authorization, primary/inherited membership, and validation limits.
