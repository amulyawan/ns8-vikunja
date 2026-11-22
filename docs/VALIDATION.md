# Candidate validation — 22 November 2026

Target: Vikunja 2.8.0 in an NS8 module. This is a development candidate.

## Acceptance gates

| Gate | Required evidence | Status |
| --- | --- | --- |
| Source checks | `python3 tools/validate.py` and clean diff | 30 tests cover module, PostgreSQL major guard and private-registry policies; see current CI result |
| NS8 settings frontend | Immutable Yarn install and production build | Passed locally on Windows Node 24.17.0 and in Linux Node 26.13.0 image build (RC2); inherited peer/deprecation/bundle-size warnings remain |
| Package build | Successful Buildah module build; image labels contain Vikunja 2.8.0 | Passed on private Linux runner, current RC2 run 44963956861 |
| Container scans | Trivy output for module **and service images** | Module, Vikunja and patched PostgreSQL 19 all passed HIGH/CRITICAL gate, current RC2 run 44963956861 |
| Private publication | Package API visibility after push | Both final packages verified private, current RC2 run 44963956861 |
| Application + PostgreSQL 19 | Synthetic user/project/task, restart, dump/restore | Passed in disposable Podman pod, run 44959585748 |
| Fresh NS8 install | Configure, frontend, API v2 version, database health, restart | Passed on NS8 3.25.1 / Rocky 9.8 / Podman 5.8.2 / SELinux Enforcing |
| MCP | Anonymous rejection, scoped initialize/tools/list/tasks_read, write denial | Passed on actual NS8; disposable MCP token revoked |
| Application flows | Two synthetic users, projects/tasks, attachment upload/download and access denial | Passed for login/CRUD/attachments and cross-user denial; comments/sharing remain untested |
| Persistence | Same synthetic records and attachment checksum after restart and web update | Passed on actual NS8 |
| Upgrade | Real 1.1.0 synthetic PostgreSQL dataset migrated to 2.8.0 | Pending staging NS8 |
| Backup/restore | Native NS8 backup into a separate restored instance; compare login/task/file/DB credentials/secret | Passed; original retained, restore copy removed after verification |
| Mail | Change smarthost; restart; verify real test delivery and TLS mode | Pending staging NS8 |
| Rollback | Restore pre-update RC1 backup to separate instance while source runs RC2 | Passed for this candidate update; legacy 1.1.0 rollback remains untested |
| Software Center | Private package pull and RC1→RC2 update from browser | Passed; synthetic data and identity preserved |
| Catalog logo | Rendered Applications image loads with correct dimensions | Passed, 256×256; fixed duplicated app directory in URL |

The Robot suite has been extended with API v2 version/health and anonymous MCP
rejection, but has not run on NS8. It does not prove upgrade, backup/restore, or
authenticated MCP. Never use it as the only production acceptance criterion.
The existing NS8 node was accessed for a separate synthetic test application.
The 17 pre-existing module versions and digests were unchanged; no failed root
systemd units were present at final verification. Test routing stays loopback
allowlisted and outbound mail is disabled for the test instance.

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
database dump/restore, frontend/API health and anonymous MCP rejection. Actual NS8 backup/restore and authenticated MCP have now passed. Migration of
an existing legacy instance remains a separate staging gate. See https://vikunja.io/docs/installing/ and
https://www.postgresql.org/about/news/postgresql-19-released-3449/ .

## Earlier candidate publication (superseded tag)

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
That run preceded the actual NS8 installation described below.


## Actual NS8 acceptance and RC2 — 22 November 2026

[RC1 run 44962299371](https://github.com/amulyawan/ns8-vikunja-build/actions/runs/44962299371)
fixed the NS8 image environment variable (`VIKUNJA_POSTGRES_IMAGE`) and introduced
a semantic module tag. A clean normal add-module/configure succeeded using the
native node registry authfile. Login/project/task/attachment and restart passed.
A second user was denied access to the first user's task and attachment.

Native NS8 backup was restored using cluster restore-module, `replace:false`,
into a separate agent. Login, task, attachment SHA-256, database credentials and
Vikunja service secret matched. Both test routes were loopback allowlisted and
mail disabled. The restore copy was removed after verification; backup retained.
The test-only backup job is disabled. Existing production backup configuration
was not edited or run by this test.

[RC2 run 44963956861](https://github.com/amulyawan/ns8-vikunja-build/actions/runs/44963956861)
built/scanned/published the next private candidate. The browser Software Center
offered RC2 and successfully updated the original test instance from RC1. Task,
attachment checksum, database credentials and secret survived the update.

- Module: `ghcr.io/amulyawan/ns8-vikunja-build/vikunja:2.8.0-rc.2`
- Module digest: `sha256:e7c1d9fd78a38e33f0ba69fa0429490961d1beb6d2c3d87d5739866fd8f07a23`
- Application: Vikunja 2.8.0; database: patched PostgreSQL 19.0.
- Software Center catalog and rendered 256px logo verified in the browser.
- MCP initialize, tools/list and tasks_read passed; read-only API token write
  was denied. Each disposable MCP token was revoked after the test.
- Private evidence, token value, screenshots and target topology remain local.

This accepts the tested fresh-install and candidate-update scope. It does not
claim legacy 1.1.0 data migration, production TLS/DNS, actual mail delivery,
comments/sharing flows, node reboot or all-severity vulnerability freedom.


## RC4 network policy and LDAP configuration — 22 November 2026

[Run 44969884340](https://github.com/amulyawan/ns8-vikunja-build/actions/runs/44969884340)
built, scanned and privately published `2.8.0-rc.4`. Source suite: 30 tests.
The preceding RC3 run initially exposed a PostgreSQL startup-probe race: the
Unix-socket temporary init server accepted pg_isready before shutting down.
The probe now waits for TCP readiness; the corrected publication gates passed.

Actual NS8 update to RC4 passed. User domain discovery/binding, native LDAP
bind/search from the pod network, HTTP 200 through trusted HTTPS and API health
passed. The owner confirmed successful login with their own AD credentials;
no end-user password was collected. Local synthetic accounts were disabled via
the supported CLI and now fail login with HTTP 412/code 1020. A nonexistent AD
user is rejected with 403 and public registration with 404. Original DB secret
files and attachment SHA-256 were preserved.

The instance's public access was explicitly owner-authorized. No LDAP, database,
or other application route was exposed. A separate owner-approved, backed-up
mail-attribute reconciliation used the native Samba partial-update API; sensitive
user inventories and its before/after evidence remain private. Unchanged group,
identity, account status and password-change timestamps were verified.

The import settings page rendered successfully in the authenticated browser
after the update restart. A transient Bad Gateway report coincided with the
restart; sustained health and frontend requests returned 200 afterward. An actual
import file/job has not been submitted. Team user lookup requires an existing
Vikunja profile; LDAP login creates it lazily. Read-only source/runtime checks
for new NS8 users are documented in LDAP-DAN-AKSES-NS8-ID.md.

The user-domain/attribute select display was found to need explicit selected
option bindings for asynchronously loaded settings; the source fix is included
in the next image build. No claim of pre-login LDAP bulk provisioning is made
for RC4.

## RC5 pre-login provisioning, directory teams and mandatory discovery

[Run 44975299796](https://github.com/amulyawan/ns8-vikunja-build/actions/runs/44975299796)
built, tested, scanned and privately published `2.8.0-rc.5` from source `8aa2320`.
Module digest: `sha256:5650703012f93540c2238110f603a7880fc0344fb7f73fdd21b9aa3a8c221332`.
The source suite passed 31 tests; pinned native-model tests passed, including
identity reuse, disabled profiles, primary/nested membership, renames, revocation
and preservation of manual teams/projects. The rebuilt backend/embedded frontend
also passed a disposable local SQLite policy smoke. CI exercised PostgreSQL 19
and proved a direct API PUT with both discovery fields false leaves both true.
The first policy CI attempt stopped before scans/publication because its test
incorrectly used POST; the test was corrected to the documented API v2 PUT.
Earlier narrower workflows were cancelled before publication when scope expanded.

Actual NS8 update and dry-run passed. Provisioning created the planned profiles
without local passwords or admin flags. All eligible LDAP identities and primary/
nested memberships exactly matched an independently read directory snapshot.
A repeated synchronization did not duplicate users, projects, teams or membership.
The timer is active/enabled; secret files, existing data and synthetic attachment
SHA-256 were preserved. Original application versions/digests were unchanged.
Local test accounts remain disabled, registration remains closed, and instance
SMTP remains disabled. Public trusted HTTPS/API health passed.

The authenticated browser visibly showed the automatically created LDAP teams
and both Privacy checkboxes checked and disabled, with the organization-policy
notice. User-domain selection now displays correctly. The email-attribute selector
still renders blank despite the backend retaining mail; this UI issue is recorded
for the next candidate, so no UI-setting round-trip is claimed for that selector.
Private screenshots and counts remain owner-local.

RC5 uses the official container base but overlays a source-built web executable
with the documented policy patch. It does not claim an unchanged official web
binary. Current directory synchronization is read-only. Authorized AD membership
changes from Vikunja UI are the next, separate scope; projects remain independent.
The prior native restore test was RC2; a new full RC5 LDAP/team restore has not
been run. No real import job, SMTP delivery or legacy 1.1.0 migration is claimed.
