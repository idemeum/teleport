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
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

func TestIdemeumSamlConnector(t *testing.T) {

	connector, err := NewIdemeumSamlConnector("https://example.idemeum.com")

	require.Nil(t, err)
	require.Empty(t, connector.GetSigningKeyPair())

	require.Equal(t, connector.GetAssertionConsumerService(), "https://example.remote.idemeum.com/v1/webapi/saml/acs")
	require.Equal(t, connector.GetIssuer(), "https://example.idemeum.com/api/saml/metadata")
	require.Equal(t, connector.GetEntityDescriptorURL(), "https://example.idemeum.com/api/saml/metadata")
	require.Equal(t, connector.GetServiceProviderIssuer(), "https://example.remote.idemeum.com/v1/webapi/saml/acs")
	require.Equal(t, connector.GetSSO(), "https://example.idemeum.com/saml/signon")
}

func TestIdemeumAdminRole(t *testing.T) {
	role := NewIdemeumAdminRole()

	require.Equal(t, role.GetMetadata().Name, "ADMIN")
	validateRole(t, role)
}

func TestIdemeumUserRole(t *testing.T) {
	role := NewIdemeumUserRole()

	require.Equal(t, role.GetMetadata().Name, "USER")
	validateRole(t, role)
}

func validateRole(t *testing.T, role types.Role) {
	require.Equal(t, role.GetAppLabels(true), types.Labels{"idemeum_app_id": []string{"{{external.app_ids}}"}})
	require.Equal(t, role.GetNodeLabels(true), types.Labels{"idemeum_app_id": []string{"{{external.node_ids}}"}})
	require.Equal(t, role.GetIdemeumEntitlements(), []string{"{{external.idemeum_entitlements}}"})
}
