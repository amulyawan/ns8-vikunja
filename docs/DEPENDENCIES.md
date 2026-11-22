# Dependency and provenance record

Checked 2026-11-22. Decision: development candidate only; production depends on
successful package build, transitive dependency review, vulnerability scans and
NS8 acceptance. This record is not a declaration that every image is CVE-free.

| Component / role | Version | Source and official license | Commercial use / restrictions | Activation decision / fallback |
| --- | --- | --- | --- | --- |
| NS8 integration module | upstream d78ccb52a1409a3a563976efbf92096af5f9e7ac | [Source](https://github.com/geniusdynamics/ns8-vikunja), [GPL-3.0](https://github.com/geniusdynamics/ns8-vikunja/blob/main/LICENSE); source headers say GPL-3.0-or-later | allowed subject to GPL source/license obligations; retain attribution | local derivative; rollback to upstream commit |
| Vikunja application | 2.8.0 | [Release](https://github.com/go-vikunja/vikunja/releases/tag/v2.8.0), [AGPL-3.0-or-later](https://github.com/go-vikunja/vikunja/blob/v2.8.0/LICENSE) | allowed subject to AGPL obligations, including source availability for modified network versions; paid Pro features are outside scope | official image; no application source modifications; fallback to prior complete snapshot |
| PostgreSQL database | 19.0-alpine3.24, latest stable 2026-11-22 | [Official image](https://hub.docker.com/_/postgres), [PostgreSQL License](https://www.postgresql.org/about/licence/) | allowed with notices; base image has additional component licenses | latest fresh-install baseline; legacy major upgrades require logical migration |
| Patched PostgreSQL runtime | private vikunja-postgres:19.0-ns8.1 | `containers/postgres/Containerfile`; pinned official base above | preserve upstream licensing; Alpine package licenses and notices retained | own stack compatibility test and full scan required before publication |
| gosu privilege helper | unchanged 1.20 source, commit 29b939aa5bd4dc588af765b8248f6e39150d2565 | [Source](https://github.com/tianon/gosu/tree/1.20), [Apache-2.0](https://github.com/tianon/gosu/blob/1.20/LICENSE) | commercial use allowed with notices; license copied into runtime | rebuild with patched Go, test privilege drop in container |
| Go helper build toolchain | 1.27.4-alpine3.24, index sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 | [Official image](https://hub.docker.com/_/golang), [BSD-3-Clause](https://go.dev/LICENSE) | commercial use allowed with notices | build-only; source deps checked against go.sum; full derived-image scan required |
| Node build runtime | 26.13.0-slim image, inherited upstream pin | [Official image](https://hub.docker.com/_/node), [MIT and bundled licenses](https://github.com/nodejs/node/blob/main/LICENSE) | allowed subject to notices | build-only; local UI check uses installed 24.17.0, so runner build still required |
| Corepack build helper | 0.36.0 | [Source/license](https://github.com/nodejs/corepack/blob/main/LICENSE.md), npm registry metadata | MIT; commercial use allowed | build-only, exact version; cache can be rebuilt |
| Yarn build helper | 4.18.1 | [Source/license](https://github.com/yarnpkg/berry/blob/master/LICENSE.md), npm registry metadata | BSD-2-Clause; commercial use allowed | immutable upstream lockfile; cache can be rebuilt |
| Buildah / Podman | distro/runner supplied; record actual build versions | [Buildah Apache-2.0](https://github.com/containers/buildah/blob/main/LICENSE), [Podman Apache-2.0](https://github.com/containers/podman/blob/main/LICENSE) | allowed with notices | not installed by this task; use trusted Linux runner; no Docker Desktop dependency |
| GitHub checkout action | v7, commit 3d3c42e5aac5ba805825da76410c181273ba90b1 | [MIT](https://github.com/actions/checkout/blob/main/LICENSE) | commercial use allowed with notices | source checkout in isolated runner |
| Trivy action / scanner | action v0.36.0, commit ed142fd0673e97e23eac54620cfb913e5ce36c25; scanner v0.70.0 | [Action Apache-2.0](https://github.com/aquasecurity/trivy-action/blob/master/LICENSE), [Scanner Apache-2.0](https://github.com/aquasecurity/trivy/blob/main/LICENSE) | commercial use allowed with notices | candidate scans before registry push; fail on HIGH/CRITICAL |

The inherited `NethServer/ns8-github-actions` repository had no root license file
or GitHub license API result at inspection (HTTP 404). No commercial-use permission
is inferred. Its reusable publish/docs/cleanup/QEMU workflows were removed from
this candidate; the candidate publish workflow is independently authored. NS8
acceptance remains a manual staging gate. Its observed v1 revision was
`ffd0c0cbcc4960a140d2999a8a89673530263197`; none of its implementation is vendored.

## Registry evidence

Docker Hub manifests were queried directly over HTTPS; these are multi-platform
index digests, **observed rather than enforced digest pins**. Tags can move;
record the actual pulled image digests at build/deployment. A checksum is not a
signature or a vulnerability scan.

| Image | Observed index digest |
| --- | --- |
| docker.io/vikunja/vikunja:2.8.0 | sha256:35d937baf3b5d5803bed6b9e11d007448b7b536df9e4346dc35551e1072636c3 |
| docker.io/library/postgres:15.20-alpine3.24 | sha256:e18480a22d6e48f4d816a6696769b1ec09ac73a824c5649e49c7d8b684913f3d |
| docker.io/library/postgres:19.0-alpine3.24 (current base) | sha256:8b0ff4cdf221f0c53d7fcda681d3b461be725c492cd881c5ec9c0ad25023f188 |
| docker.io/library/node:26.13.0-slim | sha256:5adad64f9a1fca2d51972caa65983634c8a774eb300d72b20f7f2a0f60368072 |

Vikunja, PostgreSQL and Node tags all advertised Linux amd64 and arm64 manifests.
This does not prove that the NS8 module runner or a target host supports both.

## Remaining supply-chain gates

The optional LDAP profile companion is compiled against the unchanged Vikunja
2.8.0 core at commit `6e788bf87cebe50e2b8d034b213fcf94ae5449ee`.
The downloaded source archive is verified against SHA256
`120642eb6355871bedd1fe149af7e747c4686270d1cffb9d9c173c9b07c59217`.
Integration source is AGPL-3.0-or-later, commercially usable subject to its source
and notice obligations. The public repository supplies the integration source
and reproducible build script; the pinned upstream source is available from
[Vikunja](https://github.com/go-vikunja/vikunja/tree/6e788bf87cebe50e2b8d034b213fcf94ae5449ee).
The official image remains the container base, but its executable is overlaid
with a reproducible source build carrying the documented mandatory name/email
discovery policy in `apply-policy.py`. The Vue frontend has both controls locked;
creation/update models and a one-time migration enforce the same policy.
Native license checks are retained; this does not enable Pro features or grant
global admin. No new frontend dependency is added; the upstream immutable pnpm
lockfile and its commercially usable open-source dependency set are reused.
Go and dependency notices are included under `share/ldap-sync-licenses` in the
module image. The module archive scan includes both static Go executables.

Keep the upstream UI lockfile and immutable install. Review its full dependency
license report/SBOM before production, including image base packages. Scan both
the module and runtime service images. Retain security scan failures as blockers,
record applicable advisories and verify host kernel updates separately. No new
UI dependency was added in this change. GitHub account quota/hosting terms remain
the owner's existing service contract; this task does not accept paid terms.
