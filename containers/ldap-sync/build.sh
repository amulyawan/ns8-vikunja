#!/bin/bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
source_commit=6e788bf87cebe50e2b8d034b213fcf94ae5449ee
source_sha=120642eb6355871bedd1fe149af7e747c4686270d1cffb9d9c173c9b07c59217
apk add --no-cache bash curl gcc musl-dev git python3
mkdir -p /work/api /out/licenses
curl --fail --location --retry 3 "https://codeload.github.com/go-vikunja/vikunja/tar.gz/${source_commit}" -o /work/source.tar.gz
echo "${source_sha}  /work/source.tar.gz" | sha256sum -c -
tar -xzf /work/source.tar.gz -C /work/api --strip-components=1
python3 /integration/apply-policy.py /work/api
cp /integration/DirectoryGroups.vue /work/api/frontend/src/components/teams/DirectoryGroups.vue
cp /integration/DirectoryGroups.test.ts /work/api/frontend/src/components/teams/DirectoryGroups.test.ts
cp /integration/ns8_*.go /work/api/pkg/modules/auth/ldap/
cp /integration/directory_api*.go /work/api/pkg/routes/api/v2/
mkdir -p /work/api/cmd/ns8-ldap-sync
cp /integration/main*.go /work/api/cmd/ns8-ldap-sync/
cd /work/api
export CGO_ENABLED=1 GOTOOLCHAIN=local GOMAXPROCS=2 GOMEMLIMIT=1536MiB
go test -mod=readonly ./cmd/ns8-ldap-sync ./pkg/modules/auth/ldap -run 'Test(NS8|DirectoryFilter)'
go test -mod=readonly ./pkg/routes/api/v2 -run '^TestNS8Directory'
go build -mod=readonly -trimpath -ldflags='-s -w -linkmode external -extldflags "-static"' -o /out/ns8-ldap-sync ./cmd/ns8-ldap-sync
mkdir -p cmd/ns8-directory-storage-check
cp /integration/storage_main.go cmd/ns8-directory-storage-check/main.go
go build -mod=readonly -trimpath -tags ns8storagetest -ldflags='-s -w -linkmode external -extldflags "-static"' -o /out/ns8-directory-storage-check ./cmd/ns8-directory-storage-check
if [[ "${SKIP_FRONTEND_SOURCE:-0}" != 1 ]]; then
  mkdir -p /out/frontend
  cp -a /work/api/frontend/. /out/frontend/
fi
cp LICENSE /out/licenses/Vikunja-AGPL-3.0.txt
cp /usr/local/go/LICENSE /out/licenses/Go-BSD-3-Clause.txt
# Retain dependency license notices for this statically linked executable.
go list -mod=readonly -m -f '{{if .Dir}}{{.Path}}|{{.Dir}}{{end}}' all > /work/modules
while IFS='|' read -r module directory; do
  test -n "$directory" || continue
  destination="/out/licenses/dependencies/${module}"
  for notice in "$directory"/LICENSE* "$directory"/COPYING* "$directory"/NOTICE*; do
    test -f "$notice" || continue
    mkdir -p "$destination"
    cp "$notice" "$destination/"
  done
done < /work/modules
printf '%s\n' "Upstream commit: ${source_commit}" "Source SHA256: ${source_sha}" > /out/licenses/SOURCE.txt
