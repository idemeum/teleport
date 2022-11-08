/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"net/url"
	"strings"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

func NewIdemeumAdminRole() types.Role {
	return idemeumRole("ADMIN", true)
}

func NewIdemeumUserRole() types.Role {
	return idemeumRole("USER", false)
}

func NewIdemeumSamlConnector(idemeumTenantUrl string) (types.SAMLConnector, error) {
	acsUrl, err := getACSUrl(idemeumTenantUrl)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	issuerUrl := idemeumTenantUrl + "/api/saml/metadata"
	ssoUrl := idemeumTenantUrl + "/saml/signon"
	return &types.SAMLConnectorV2{
		Kind:    types.KindSAMLConnector,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "idemeum-saml",
		},
		Spec: types.SAMLConnectorSpecV2{
			AssertionConsumerService: acsUrl,
			AttributesToRoles: []types.AttributeMapping{
				{
					Name:  "roles",
					Value: "USER",
					Roles: []string{"USER"},
				},
				{
					Name:  "roles",
					Value: "ADMIN",
					Roles: []string{"ADMIN"},
				},
			},
			Audience:              acsUrl,
			Display:               "Idemeum",
			EntityDescriptorURL:   issuerUrl,
			Issuer:                issuerUrl,
			ServiceProviderIssuer: acsUrl,
			SSO:                   ssoUrl,
		},
	}, nil
}

func idemeumRole(roleName string, admin bool) types.Role {
	role := &types.RoleV5{
		Kind:    types.KindRole,
		Version: types.V5,
		Metadata: types.Metadata{
			Name:        roleName,
			Namespace:   apidefaults.Namespace,
			Description: "Idemeum remote access resources",
		},
		Spec: types.RoleSpecV5{
			Options: types.RoleOptions{
				CertificateFormat: constants.CertificateFormatStandard,
				MaxSessionTTL:     types.NewDuration(apidefaults.MaxCertDuration),
				PortForwarding:    types.NewBoolOption(true),
				ForwardAgent:      types.NewBool(true),
				BPF:               apidefaults.EnhancedEvents(),
				RecordSession:     &types.RecordSession{Desktop: types.NewBoolOption(true)},
			},
			Allow: types.RoleConditions{
				Namespaces:           []string{apidefaults.Namespace},
				NodeLabels:           types.Labels{"idemeum_app_id": []string{"{{external.node_ids}}"}},
				AppLabels:            types.Labels{"idemeum_app_id": []string{"{{external.app_ids}}"}},
				WindowsDesktopLabels: types.Labels{"idemeum_app_id": []string{"{{external.win_ids}}"}},
				Rules:                getRoleRules(admin),
				IdemeumEntitlements:  []string{"{{external.idemeum_entitlements}}"},
			},
		},
	}
	role.SetLogins(types.Allow, []string{"{{external.node_users}}"})
	role.SetWindowsLogins(types.Allow, []string{"{{external.win_users}}"})
	return role
}

func getRoleRules(admin bool) []types.Rule {
	if admin {
		return []types.Rule{
			{
				Resources: []string{types.KindSession},
				Verbs:     []string{types.VerbRead, types.VerbList},
				Where:     "contains(session.participants, user.metadata.name)",
			},
		}
	}
	return []types.Rule{
		{
			Resources: []string{types.KindSession},
			Verbs:     []string{types.VerbRead, types.VerbList},
		},
	}
}

func getACSUrl(tenantUrl string) (string, error) {
	u, err := url.Parse(tenantUrl)
	if err != nil {
		return "", trace.BadParameter("invalid idemeum tenant url")
	}
	remoteAccessServerFqdn := getRemoteAccessServerFqdn(u.Hostname())
	if remoteAccessServerFqdn == "" {
		return "", trace.BadParameter("invalid idemeum tenant url")
	}

	return "https://" + remoteAccessServerFqdn + "/v1/webapi/saml/acs", nil
}

func getRemoteAccessServerFqdn(hostName string) string {
	hostNameParts := strings.Split(hostName, ".")
	if len(hostNameParts) < 3 {
		return ""
	}

	newHostNameParts := make([]string, len(hostNameParts)+1)
	newHostNameParts[0] = hostNameParts[0]
	newHostNameParts[1] = "remote"
	index := 2
	for i := 1; i < len(hostNameParts); i++ {
		newHostNameParts[index] = hostNameParts[i]
		index++
	}

	return strings.Join(newHostNameParts, ".")
}
