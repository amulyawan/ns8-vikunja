#!/usr/bin/env python3
"""Apply the documented mandatory discoverability policy to pinned 2.8.0 source."""
# SPDX-License-Identifier: AGPL-3.0-or-later
import json
from pathlib import Path
import sys

root = Path(sys.argv[1])

def replace(relative, old, new, count=1):
    path = root / relative
    source = path.read_text()
    if source.count(old) != count:
        raise RuntimeError("Pinned-source policy anchor changed: " + relative)
    path.write_text(source.replace(old, new))

replace("pkg/user/user_create.go",
        "user.DiscoverableByName = config.DefaultSettingsDiscoverableByName.GetBool()",
        "user.DiscoverableByName = true // NS8 organization policy")
replace("pkg/user/user_create.go",
        "user.DiscoverableByEmail = config.DefaultSettingsDiscoverableByEmail.GetBool()",
        "user.DiscoverableByEmail = true // NS8 organization policy")
replace("pkg/user/user.go",
        "func UpdateUser(s *xorm.Session, user *User, forceOverride bool) (updatedUser *User, err error) {",
        "func UpdateUser(s *xorm.Session, user *User, forceOverride bool) (updatedUser *User, err error) {\n"
        "\t// Mandatory instance policy, including direct API requests.\n"
        "\tuser.DiscoverableByName = true\n\tuser.DiscoverableByEmail = true")

for field in ("name", "email"):
    anchor = 'v-model="settings.discoverable_by_' + field + '"'
    replace("frontend/src/views/user/settings/GeneralSettingsForm.vue", anchor,
            anchor + '\n\t\t\t\t:disabled="true"')
replace("frontend/src/views/user/settings/GeneralSettingsForm.vue",
        '<div class="field-group">\n\t\t\t<FormCheckbox\n\t\t\t\tv-model="settings.discoverable_by_name"',
        '<p>{{ $t(\'user.settings.general.ns8DiscoveryPolicy\') }}</p>\n\t\t<div class="field-group">\n\t\t\t<FormCheckbox\n\t\t\t\tv-model="settings.discoverable_by_name"')
path = root / "frontend/src/i18n/lang/en.json"
messages = json.loads(path.read_text())
messages["user"]["settings"]["general"]["ns8DiscoveryPolicy"] = "Name and full-email discovery are required by your organization and cannot be disabled."
path.write_text(json.dumps(messages, ensure_ascii=False, indent=2) + "\n")
(root / "frontend/src/version.json").write_text('{"VERSION":"2.8.0-ns8.1"}\n')

# Migrate existing profiles once, before the application accepts requests.
(root / "pkg/migration/20261122090001.go").write_text('''// SPDX-License-Identifier: AGPL-3.0-or-later
package migration
import (
    "src.techknowlogick.com/xormigrate"
    "xorm.io/xorm"
)
func init() {
    migrations = append(migrations, &xormigrate.Migration{
        ID: "20261122090001",
        Description: "NS8 mandatory name and full-email discovery policy",
        Migrate: func(tx *xorm.Engine) error {
            _, err := tx.Exec("UPDATE users SET discoverable_by_name = true, discoverable_by_email = true")
            return err
        },
        Rollback: func(tx *xorm.Engine) error { return nil },
    })
}
''')
