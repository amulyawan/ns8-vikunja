// SPDX-License-Identifier: AGPL-3.0-or-later
// Compiled alongside the pinned Vikunja LDAP package; not part of the web binary.
package ldap

import (
	"strings"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/license"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"
	"github.com/asaskevich/govalidator"
	"github.com/go-ldap/ldap/v3"
	"xorm.io/xorm"
)

// NS8ProvisionEntry uses the same immutable identity and creation models as an
// ordinary first LDAP login. It never creates a local password or grants admin.
func NS8ProvisionEntry(s *xorm.Session, entry *ldap.Entry, apply bool) (state string, id int64, err error) {
	username := entry.GetEqualFoldAttributeValue(config.AuthLdapAttributeUsername.GetString())
	email := strings.TrimSpace(entry.GetEqualFoldAttributeValue(config.AuthLdapAttributeEmail.GetString()))
	name := entry.GetEqualFoldAttributeValue(config.AuthLdapAttributeDisplayname.GetString())
	if username == "" || !govalidator.IsEmail(email) || config.AuthLdapAttributeID.GetString() == "" {
		return "invalid", 0, nil
	}
	subject, err := directoryID(entry, config.AuthLdapAttributeID.GetString())
	if err != nil {
		return "invalid", 0, nil
	}
	u, err := user.GetUserWithEmail(s, &user.User{Issuer: user.IssuerLDAP, Subject: subject})
	legacy := false
	if user.IsErrUserDoesNotExist(err) && subject != username {
		u, err = user.GetUserWithEmail(s, &user.User{Issuer: user.IssuerLDAP, Subject: username})
		legacy = err == nil || user.IsErrUserStatusError(err)
	}
	if user.IsErrUserStatusError(err) {
		return "disabled", u.ID, nil
	}
	if err != nil && !user.IsErrUserDoesNotExist(err) {
		return "error", 0, err
	}
	if err == nil {
		if u.Email == email && u.Name == name && !legacy {
			return "existing", u.ID, nil
		}
		if !apply {
			return "would_update", u.ID, nil
		}
		if legacy {
			u, err = findOrMigrateLdapUser(s, subject, username)
			if err != nil {
				return "error", 0, err
			}
		}
		u.Email, u.Name = email, name
		_, err = user.UpdateUser(s, u, false)
		return "updated", u.ID, err
	}
	username = strings.ReplaceAll(username, " ", "-")
	_, taken := user.GetUserByUsername(s, username)
	if taken == nil || user.IsErrUserStatusError(taken) {
		return "username_collision", 0, nil
	}
	if !user.IsErrUserDoesNotExist(taken) {
		return "error", 0, taken
	}
	if license.MaxUsersReached() {
		return "license_limit", 0, nil
	}
	if !apply {
		return "would_create", 0, nil
	}
	u, err = user.CreateUser(s, &user.User{
		Username: username, Email: email, Name: name,
		Issuer: user.IssuerLDAP, Subject: subject,
	})
	if err != nil {
		return "error", 0, err
	}
	if err = models.CreateNewProjectForUser(s, u); err != nil {
		return "error", 0, err
	}
	return "created", u.ID, nil
}
