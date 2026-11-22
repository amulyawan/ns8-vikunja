#!/bin/bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
if [[ ! -d /work/api ]]; then
  SKIP_FRONTEND_SOURCE=1 bash /integration/build.sh
fi
cp -a /out/frontend/dist /work/api/frontend/dist
cd /work/api
export CGO_ENABLED=1 GOTOOLCHAIN=local GOMAXPROCS=2 GOMEMLIMIT=1536MiB
go build -mod=readonly -trimpath -tags 'osusergo netgo' \
  -ldflags='-s -w -linkmode external -extldflags "-static" -X code.vikunja.io/api/pkg/version.Version=2.8.0 -X "main.Tags=osusergo netgo ns8-directory-policy-v1"' \
  -o /out/vikunja .
