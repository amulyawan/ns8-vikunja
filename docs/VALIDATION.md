# Candidate validation — 22 November 2026

Target: Vikunja 2.8.0 in an NS8 module. This is a development candidate.

## Acceptance gates

| Gate | Required evidence | Status |
| --- | --- | --- |
| Source checks | `python3 tools/validate.py` and clean diff | Passed locally: 9 contract tests; Python, Bash, JSON parsing |
| NS8 settings frontend | Immutable Yarn install and production build | Passed locally on Windows Node 24.17.0; inherited peer/deprecation/bundle-size warnings; Linux Node 26 build still pending |
| Package build | Successful Buildah module build; image labels contain Vikunja 2.8.0 | Pending Linux Buildah runner |
| Container scans | Trivy output for module **and service images** | Pending registry build |
| Fresh NS8 install | Configure, frontend, API v2 version, database health, restart | Pending staging NS8 |
| MCP | Anonymous rejection plus authenticated initialize/tools/list with scoped token | Pending staging NS8 |
| Application flows | Two synthetic users, projects, tasks, comments, sharing, attachment upload/download | Pending staging NS8 |
| Persistence | Same synthetic records and attachments after restart | Pending staging NS8 |
| Upgrade | Real 1.1.0 synthetic PostgreSQL dataset migrated to 2.8.0 | Pending staging NS8 |
| Backup/restore | Restore NS8 backup into separate instance; compare users, tasks, files, stable secret | Pending staging NS8 |
| Mail | Change smarthost; restart; verify real test delivery and TLS mode | Pending staging NS8 |
| Rollback | Restore pre-upgrade snapshot/backup and previous application version | Pending staging NS8 |

The Robot suite has been extended with API v2 version/health and anonymous MCP
rejection, but has not run on NS8. It does not prove upgrade, backup/restore, or
authenticated MCP. Never use it as the only production acceptance criterion.
No production instance has been accessed.

The official Linux amd64 2.8.0 release binary was downloaded from GitHub Releases
and its archive SHA-256 matched the published asset digest:
`0b6ba2934bd432eefaa4209b0f96d20b7f7cd7f7be0269dd79071979c4363423`.
Its frontend, API version, health and anonymous MCP rejection passed on a temporary
loopback-only process using synthetic SQLite data. The process was stopped.
This is application-level diagnostic evidence; it does **not** validate the NS8
container, PostgreSQL, rootless volume permissions, TLS or migration from 1.1.0.

Runtime compatibility cannot be inferred from changing an image label. PostgreSQL
stays on upstream's major version 15; do not combine a database major upgrade with
this application change. Do not downgrade an already migrated database in place.
