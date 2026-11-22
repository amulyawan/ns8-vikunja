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
messages["team"]["ns8Directory"] = {
    "title": "Manage directory group members",
    "help": "Teams follow directory groups. Your directory verifies your permissions for each change. Projects are managed separately in Vikunja.",
    "enableSync": "Enable automatic LDAP user and group synchronization in NS8 before managing directory groups here.",
    "password": "Your directory password",
    "checkPermissions": "Load groups I can manage",
    "noPermission": "Your directory account does not have permission to manage membership of any visible group.",
    "group": "Directory group",
    "selectGroup": "Select a group",
    "person": "Member",
    "membership": "Membership",
    "direct": "Direct",
    "primary": "Primary group",
    "inherited": "Inherited",
    "remove": "Remove",
    "membershipHelp": "Only direct members can be removed here. Primary and inherited memberships must be changed at their source. Create new groups in your directory administration interface.",
    "confirmAdd": "Add {name} to {group}?",
    "confirmRemove": "Remove {name} from {group}?",
    "confirmHelp": "This changes group membership in your directory. Use your own directory password to confirm. Your password is not stored.",
    "confirm": "Confirm directory change",
    "updated": "Directory membership updated and synchronized to Vikunja.",
    "refreshPending": "The directory was updated. Vikunja refresh is pending; reload group permissions before making another change.",
    "unconfirmed": "The directory result could not be confirmed. Reload group permissions and review membership before retrying.",
    "inheritedRemains": "Direct membership was removed. The user still inherits membership through another group.",
    "failed": "The directory request could not be completed.",
}
path.write_text(json.dumps(messages, ensure_ascii=False, indent=2) + "\n")
(root / "frontend/src/version.json").write_text('{"VERSION":"2.8.0-ns8.2"}\n')

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

(root / "pkg/migration/20261122090002.go").write_text('''// SPDX-License-Identifier: AGPL-3.0-or-later
package migration
import (
    "time"
    "src.techknowlogick.com/xormigrate"
    "xorm.io/xorm"
)
type ns8DirectoryMemberRecovery struct {
    ID int64 `xorm:"pk autoincr"`
    ActorID int64
    ActorSubject string
    GroupExternalID string
    TargetID int64
    TargetSubject string
    BeforeDirect bool
    AfterDirect bool
    Status string
    Created time.Time `xorm:"created"`
    Updated time.Time `xorm:"updated"`
}
func (ns8DirectoryMemberRecovery) TableName() string { return "ns8_directory_member_changes" }
func ns8InitDirectoryStorage(tx *xorm.Engine) error {
    if err := tx.Sync2(new(ns8DirectoryMemberRecovery)); err != nil { return err }
    if _,err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS ns8_ldap_team_identity_unique ON teams (issuer,external_id) WHERE issuer='ldap' AND external_id IS NOT NULL AND external_id<>''");err!=nil{return err}
    _,err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS ns8_ldap_identity_unique ON users (issuer,subject) WHERE issuer='ldap' AND subject IS NOT NULL AND subject<>''")
    return err
}
func init() {
    migrations = append(migrations, &xormigrate.Migration{
        ID: "20261122090002",
        Description: "NS8 directory membership recovery journal and identity guard",
        Migrate: ns8InitDirectoryStorage,
        Rollback: func(tx *xorm.Engine) error { return nil },
    })
}
''')
replace("pkg/migration/migration.go", "m.InitSchema(initSchema)",
        "m.InitSchema(func(engine *xorm.Engine) error {\n"
        "\t\tif err := initSchema(engine); err != nil { return err }\n"
        "\t\treturn ns8InitDirectoryStorage(engine)\n\t})")

# In directory mode, membership changes must use the actor-authorized AD path.
# Internal native directory sync bypasses these HTTP permission methods.
for relative, signatures in {
    "pkg/models/teams_permissions.go": [
        "func (t *Team) CanCreate(_ *xorm.Session, a web.Auth) (bool, error) {",
        "func (t *Team) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {",
        "func (t *Team) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {",
    ],
    "pkg/models/team_members_permissions.go": [
        "func (tm *TeamMember) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {",
        "func (tm *TeamMember) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {",
        "func (tm *TeamMember) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {",
    ],
}.items():
    replace(relative, 'import (', 'import (\n\t"code.vikunja.io/api/pkg/config"')
    for signature in signatures:
        replace(relative, signature, signature + '\n\tif config.AuthLdapEnabled.GetBool() { return false, nil }')

# Teams remain a projection of AD when LDAP is selected. The same management
# component is available from the team list and an individual external team.
replace("frontend/src/views/teams/ListTeams.vue",
        '\t\t<XButton\n\t\t\t:to="{name:\'teams.create\'}"',
        '\t\t<DirectoryGroups v-if="configStore.auth.ldap.enabled" />\n\t\t<XButton\n\t\t\tv-if="!configStore.auth.ldap.enabled"\n\t\t\t:to="{name:\'teams.create\'}"')
replace("frontend/src/views/teams/ListTeams.vue", '<RouterLink :to="{name: \'teams.create\'}">',
        '<RouterLink v-if="!configStore.auth.ldap.enabled" :to="{name: \'teams.create\'}">')
replace("frontend/src/views/teams/ListTeams.vue", "import Card from '@/components/misc/Card.vue'",
        "import Card from '@/components/misc/Card.vue'\nimport DirectoryGroups from '@/components/teams/DirectoryGroups.vue'\nimport {useConfigStore} from '@/stores/config'")
replace("frontend/src/views/teams/ListTeams.vue", "const { t } = useI18n({useScope: 'global'})",
        "const configStore = useConfigStore()\nconst { t } = useI18n({useScope: 'global'})")
replace("frontend/src/views/teams/EditTeam.vue", '\t\t<EditTeamForm',
        '\t\t<DirectoryGroups v-if="configStore.auth.ldap.enabled" :initial-group-id="team?.external_id || undefined" />\n\t\t<EditTeamForm')
replace("frontend/src/views/teams/EditTeam.vue", "import {useAuthStore} from '@/stores/auth'",
        "import {useAuthStore} from '@/stores/auth'\nimport {useConfigStore} from '@/stores/config'\nimport DirectoryGroups from '@/components/teams/DirectoryGroups.vue'")
replace("frontend/src/views/teams/EditTeam.vue", "const authStore = useAuthStore()",
        "const authStore = useAuthStore()\nconst configStore = useConfigStore()")
replace("frontend/src/views/teams/EditTeam.vue",
        "const userIsAdmin = computed(() => (team.value?.max_permission ?? PERMISSIONS.READ) === PERMISSIONS.ADMIN)",
        "const userIsAdmin = computed(() => !configStore.auth.ldap.enabled && (team.value?.max_permission ?? PERMISSIONS.READ) === PERMISSIONS.ADMIN)")
