# Candidate validation — 22 November 2026

Target: Vikunja 2.8.0 in an NS8 module. This is a development candidate.

## Acceptance gates

| Gate | Required evidence | Status |
| --- | --- | --- |
| Source checks | `python3 tools/validate.py` and clean diff | 17 tests cover module, PostgreSQL major guard and private-registry policies; see current CI result |
| NS8 settings frontend | Immutable Yarn install and production build | Passed locally on Windows Node 24.17.0; inherited peer/deprecation/bundle-size warnings; Linux Node 26 build still pending |
| Package build | Successful Buildah module build; image labels contain Vikunja 2.8.0 | Passed on private Linux runner, run 44959585748 |
| Container scans | Trivy output for module **and service images** | Module, Vikunja and patched PostgreSQL 19 all passed HIGH/CRITICAL gate, run 44959585748 |
| Private publication | Package API visibility after push | Both final packages verified private, run 44959585748 |
| Application + PostgreSQL 19 | Synthetic user/project/task, restart, dump/restore | Passed in disposable Podman pod, run 44959585748 |
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

Runtime compatibility cannot be inferred from changing an image label. The owner
requested the latest stable database for fresh installs: PostgreSQL 19.0. The
image uses its new parent volume path and a separate postgres19-data volume.
An existing legacy postgres-data volume blocks direct upgrade until a separately
verified logical migration has been completed. Never downgrade a migrated DB in place.

## First registry build and security repair

[Run 44957387237](https://github.com/amulyawan/ns8-vikunja/actions/runs/44957387237)
built commit `b6ed8bdd103a47b394974347bfe35eb78a6074c7` successfully. Trivy
passed the module and Vikunja runtime, but rejected the official PostgreSQL
15.20-alpine3.24 image: 26 HIGH and 1 CRITICAL findings across libxml2 and the
Go standard library embedded in gosu. No publication step ran.

Repair: retain the pinned official PostgreSQL 15.20 base, upgrade Alpine packages,
and rebuild unchanged gosu 1.20 from pinned source using Go 1.27.4. Publish this
as the separate **private** `vikunja-postgres:15.20-ns8.1` runtime only after its
scan and synthetic initialization/restart/dump smoke test pass. NS8 install,
old-data migration and full backup/restore still require a staging node.

## Privacy correction

Run 44957983454 passed the rebuilt PostgreSQL smoke test and all three scans.
Publication succeeded, but its final visibility check failed: both newly created
packages were publicly pullable. This did not satisfy the owner's private-image
requirement. The cause of that unexpected initial visibility was not established.

[Withdrawal run 44958344679](https://github.com/amulyawan/ns8-vikunja/actions/runs/44958344679)
deleted only those two newly created, single-version public candidates. Fresh
anonymous registry requests then returned HTTP 403. No private server configs,
owner data or credentials were included in the images; whether someone downloaded
the public candidate during that interval is unknown.

Replacement publication uses the private `amulyawan/ns8-vikunja-build` repository
and a new `ghcr.io/amulyawan/ns8-vikunja-build/` namespace. A repository-private
guard now runs before building, package visibility is checked before and after
push, and an unintended newly created public candidate is withdrawn automatically.

## Latest database decision

PostgreSQL 19.0 was released on 2026-11-19. Vikunja's public installation docs
list PostgreSQL support without a maximum server version, but their CI currently
tests 18. Therefore 19 is a candidate combination requiring our own evidence,
not a claim of upstream certification. The owner rejected retaining 15 merely
because it was pinned by the original integration.

The candidate CI now tests Vikunja 2.8.0 against PostgreSQL 19.0: automatic schema
migrations, registration/login, project/task creation, restart persistence,
database dump/restore, frontend/API health and anonymous MCP rejection. Full NS8
backup/restore, authenticated MCP and migration of an existing instance remain
separate staging gates. See https://vikunja.io/docs/installing/ and
https://www.postgresql.org/about/news/postgresql-19-released-3449/ .

## Final candidate publication

[Private run 44959585748](https://github.com/amulyawan/ns8-vikunja-build/actions/runs/44959585748)
successfully built commit `1b0e402` with Vikunja 2.8.0 and PostgreSQL 19.0.
All source checks, application persistence/dump-restore tests, three container
scans and before/after package privacy gates passed. Scans fail on HIGH/CRITICAL;
this is not a statement that all severities or host-kernel risks are absent.

- Module: `ghcr.io/amulyawan/ns8-vikunja-build/vikunja:vikunja-2.8.0`
- Database: `ghcr.io/amulyawan/ns8-vikunja-build/vikunja-postgres:19.0-ns8.1`

The first PostgreSQL 19 test had already passed initialization and application
CRUD, then failed because it restarted the application before the database.
The corrected test restarts PostgreSQL, waits for readiness, then restarts Vikunja.
No NS8 host has been modified. NS8 staging acceptance is still required.
