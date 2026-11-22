// SPDX-License-Identifier: AGPL-3.0-or-later
package ldap

import (
	"errors"
	"strings"

	"code.vikunja.io/api/pkg/config"
	"github.com/go-ldap/ldap/v3"
)

var (
	ErrNS8DirectoryCredentials = errors.New("directory credentials rejected")
	ErrNS8DirectoryPermission  = errors.New("directory does not grant membership management")
	ErrNS8DirectoryNotDirect   = errors.New("membership is inherited or primary, not a direct member value")
	ErrNS8DirectoryIncomplete  = errors.New("directory returned incomplete ranged membership")
	ErrNS8DirectoryUnavailable = errors.New("directory operation failed")
)

type NS8MemberConnection interface {
	Bind(string, string) error
	Search(*ldap.SearchRequest) (*ldap.SearchResult, error)
	Modify(*ldap.ModifyRequest) error
}

type NS8MemberChange struct {
	BeforeDirect bool
	AfterDirect  bool
	Changed      bool
}

func ns8CompleteMembership(entry *ldap.Entry) error {
	for _, attribute := range entry.Attributes {
		name := strings.ToLower(attribute.Name)
		if strings.HasPrefix(name, "member;range=") || strings.HasPrefix(name, "memberuid;range=") {
			return ErrNS8DirectoryIncomplete
		}
	}
	return nil
}

func NS8EntryID(entry *ldap.Entry) (string, error) {
	return directoryID(entry, config.AuthLdapAttributeID.GetString())
}

func ns8MemberAttribute(group *ldap.Entry) string {
	for _, class := range group.GetEqualFoldAttributeValues("objectClass") {
		if strings.EqualFold(class, "posixGroup") {
			return "memberUid"
		}
	}
	return "member"
}

func NS8MayManageGroup(group *ldap.Entry) bool {
	attribute := ns8MemberAttribute(group)
	allowed := group.GetEqualFoldAttributeValues("allowedAttributesEffective")
	for _, value := range allowed {
		if strings.EqualFold(value, attribute) {
			return true
		}
	}
	// RFC2307 servers need not implement AD's computed effective-ACL attribute.
	// In that case the real user bind and atomic Modify remain authoritative.
	return !strings.EqualFold(config.AuthLdapAttributeID.GetString(), "objectGUID") && len(allowed) == 0
}

func ns8DirectMember(group, target *ldap.Entry) bool {
	attribute := ns8MemberAttribute(group)
	wanted := target.DN
	if attribute == "memberUid" {
		wanted = target.GetEqualFoldAttributeValue(config.AuthLdapAttributeUsername.GetString())
	}
	for _, member := range group.GetEqualFoldAttributeValues(attribute) {
		if attribute == "member" && ns8DN(member) == ns8DN(wanted) || attribute == "memberUid" && strings.EqualFold(member, wanted) {
			return true
		}
	}
	return false
}

func ns8ReadWritableGroup(connection NS8MemberConnection, dn string) (*ldap.Entry, error) {
	result, err := connection.Search(ldap.NewSearchRequest(dn, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 20, false, "(objectClass=*)",
		[]string{"cn", "objectClass", "member", "memberUid", "allowedAttributesEffective", config.AuthLdapAttributeID.GetString()}, nil))
	if err != nil || len(result.Entries) != 1 {
		return nil, ErrNS8DirectoryUnavailable
	}
	group := result.Entries[0]
	if err := ns8CompleteMembership(group); err != nil {
		return nil, err
	}
	if !NS8MayManageGroup(group) {
		return nil, ErrNS8DirectoryPermission
	}
	return group, nil
}

func ns8VerifyIdentity(connection NS8MemberConnection, expected *ldap.Entry) error {
	id, err := NS8EntryID(expected)
	if err != nil {
		return ErrNS8DirectoryUnavailable
	}
	result, err := connection.Search(ldap.NewSearchRequest(expected.DN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, 20, false, "(objectClass=*)", []string{config.AuthLdapAttributeID.GetString()}, nil))
	if err != nil || len(result.Entries) != 1 {
		return ErrNS8DirectoryUnavailable
	}
	actual, err := NS8EntryID(result.Entries[0])
	if err != nil || actual != id {
		return ErrNS8DirectoryUnavailable
	}
	return nil
}

// NS8ChangeDirectMember never writes using the shared read bind. The directory
// evaluates the authenticated actor's ACL again on the single-value Modify.
// beforeWrite must durably save the recovery intent before permitting a write.
func NS8ChangeDirectMember(connection NS8MemberConnection, actor, verifiedGroup, target *ldap.Entry,
	password string, desired bool, beforeWrite func(NS8MemberChange) error) (NS8MemberChange, error) {
	var result NS8MemberChange
	if password == "" || actor == nil || actor.DN == "" || target == nil || target.DN == "" || verifiedGroup == nil {
		return result, ErrNS8DirectoryCredentials
	}
	if err := connection.Bind(actor.DN, password); err != nil {
		return result, ErrNS8DirectoryCredentials
	}
	if err := ns8VerifyIdentity(connection, actor); err != nil {
		return result, err
	}
	if err := ns8VerifyIdentity(connection, target); err != nil {
		return result, err
	}
	group, err := ns8ReadWritableGroup(connection, verifiedGroup.DN)
	if err != nil {
		return result, err
	}
	// Recheck immutable identity after binding as the actor, guarding a rename
	// or replacement between the initial lookup and the authenticated write.
	expected, err := NS8EntryID(verifiedGroup)
	if err != nil {
		return result, ErrNS8DirectoryUnavailable
	}
	actual, err := NS8EntryID(group)
	if err != nil || actual != expected {
		return result, ErrNS8DirectoryUnavailable
	}
	result.BeforeDirect = ns8DirectMember(group, target)
	result.AfterDirect = desired
	if result.BeforeDirect == desired {
		if !desired {
			return result, ErrNS8DirectoryNotDirect
		}
		return result, nil
	}
	if beforeWrite == nil {
		return result, ErrNS8DirectoryUnavailable
	}
	if err := beforeWrite(result); err != nil {
		return result, ErrNS8DirectoryUnavailable
	}
	attribute := ns8MemberAttribute(group)
	value := target.DN
	if attribute == "memberUid" {
		value = target.GetEqualFoldAttributeValue(config.AuthLdapAttributeUsername.GetString())
		if value == "" {
			return result, ErrNS8DirectoryUnavailable
		}
	}
	request := ldap.NewModifyRequest(group.DN, nil)
	if desired {
		request.Add(attribute, []string{value})
	} else {
		request.Delete(attribute, []string{value})
	}
	if err := connection.Modify(request); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInsufficientAccessRights) {
			return result, ErrNS8DirectoryPermission
		}
		return result, ErrNS8DirectoryUnavailable
	}
	result.Changed = true
	return result, nil
}
