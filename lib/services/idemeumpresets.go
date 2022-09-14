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
	return idemeumRole("USER", true)
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
				Namespaces: []string{apidefaults.Namespace},
				NodeLabels: types.Labels{"idemeum_app_id": []string{"{{external.node_ids}}"}},
				AppLabels:  types.Labels{"idemeum_app_id": []string{"{{external.app_ids}}"}},
				Rules:      getRoleRules(admin),
			},
		},
	}
	role.SetLogins(types.Allow, []string{"{{external.node_users}}"})
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
	remoteAccessUrl := ""
	if strings.Contains(tenantUrl, ".idemeum.com") {
		remoteAccessUrl = strings.ReplaceAll(tenantUrl, ".idemeum.com", ".remote.idemeum.com")
	} else if strings.Contains(tenantUrl, ".idemeumlab.com") {
		remoteAccessUrl = strings.ReplaceAll(tenantUrl, ".idemeumlab.com", ".remote.idemeumlab.com")
	} else {
		return "", trace.BadParameter("invalid idemeum tenant url")
	}
	return remoteAccessUrl + "/v1/webapi/saml/acs", nil
}
