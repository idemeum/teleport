// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package daemon

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/teleterm/gatewaytest"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type mockGatewayCreator struct {
	t         *testing.T
	callCount int
}

func (m *mockGatewayCreator) CreateGateway(ctx context.Context, params clusters.CreateGatewayParams) (*gateway.Gateway, error) {
	m.callCount++

	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	m.t.Cleanup(func() {
		hs.Close()
	})

	gateway, err := gateway.New(gateway.Config{
		LocalPort:             params.LocalPort,
		TargetURI:             params.TargetURI,
		TargetUser:            params.TargetUser,
		TargetName:            params.TargetURI,
		TargetSubresourceName: params.TargetSubresourceName,
		Protocol:              defaults.ProtocolPostgres,
		CertPath:              "../../../fixtures/certs/proxy1.pem",
		KeyPath:               "../../../fixtures/certs/proxy1-key.pem",
		Insecure:              true,
		WebProxyAddr:          hs.Listener.Addr().String(),
		CLICommandProvider:    params.CLICommandProvider,
		TCPPortAllocator:      params.TCPPortAllocator,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m.t.Cleanup(func() {
		gateway.Close()
	})

	return gateway, nil
}

type gatewayCRUDTestContext struct {
	t                    *testing.T
	nameToGateway        map[string]*gateway.Gateway
	mockGatewayCreator   *mockGatewayCreator
	mockTCPPortAllocator *gatewaytest.MockTCPPortAllocator
	daemon               *Service
}

func TestGatewayCRUD(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                 string
		gatewayNamesToCreate []string
		// tcpPortAllocator is an optional field which lets us provide a custom
		// gatewaytest.MockTCPPortAllocator with some ports already in use.
		tcpPortAllocator *gatewaytest.MockTCPPortAllocator
		testFunc         func(*gatewayCRUDTestContext)
	}{
		{
			name:                 "create then find",
			gatewayNamesToCreate: []string{"gateway"},
			testFunc: func(c *gatewayCRUDTestContext) {
				createdGateway := c.nameToGateway["gateway"]
				foundGateway, err := c.daemon.findGateway(createdGateway.URI().String())
				require.NoError(t, err)
				require.Equal(t, createdGateway, foundGateway)
			},
		},
		{
			name:                 "ListGateways",
			gatewayNamesToCreate: []string{"gateway1", "gateway2"},
			testFunc: func(c *gatewayCRUDTestContext) {
				gateways := c.daemon.ListGateways()
				gatewayURIs := map[uri.ResourceURI]struct{}{}

				for _, gateway := range gateways {
					gatewayURIs[gateway.URI()] = struct{}{}
				}

				require.Equal(t, 2, len(gateways))
				require.Contains(t, gatewayURIs, c.nameToGateway["gateway1"].URI())
				require.Contains(t, gatewayURIs, c.nameToGateway["gateway2"].URI())
			},
		},
		{
			name:                 "RemoveGateway",
			gatewayNamesToCreate: []string{"gatewayToRemove", "gatewayToKeep"},
			testFunc: func(c *gatewayCRUDTestContext) {
				gatewayToRemove := c.nameToGateway["gatewayToRemove"]
				gatewayToKeep := c.nameToGateway["gatewayToKeep"]
				err := c.daemon.RemoveGateway(gatewayToRemove.URI().String())
				require.NoError(t, err)

				_, err = c.daemon.findGateway(gatewayToRemove.URI().String())
				require.True(t, trace.IsNotFound(err), "gatewayToRemove wasn't removed")

				_, err = c.daemon.findGateway(gatewayToKeep.URI().String())
				require.NoError(t, err)
			},
		},
		{
			name:                 "RestartGateway",
			gatewayNamesToCreate: []string{"gateway"},
			testFunc: func(c *gatewayCRUDTestContext) {
				gateway := c.nameToGateway["gateway"]
				require.Equal(t, 1, c.mockGatewayCreator.callCount)

				err := c.daemon.RestartGateway(context.Background(), gateway.URI().String())
				require.NoError(t, err)
				require.Equal(t, 2, c.mockGatewayCreator.callCount)
				require.Equal(t, 1, len(c.daemon.gateways))

				// Check if the restarted gateway is still available under the same URI.
				restartedGateway, err := c.daemon.findGateway(gateway.URI().String())
				require.NoError(t, err)
				require.Equal(t, gateway.URI(), restartedGateway.URI())
			},
		},
		{
			name:                 "SetGatewayLocalPort",
			gatewayNamesToCreate: []string{"gateway"},
			testFunc: func(c *gatewayCRUDTestContext) {
				gateway := c.nameToGateway["gateway"]

				updatedGateway, err := c.daemon.SetGatewayLocalPort(context.Background(), gateway.URI().String(), "12345")
				require.NoError(t, err)
				require.Equal(t, "12345", updatedGateway.LocalPort())

				// Check if the restarted gateway is still available under the same URI.
				foundGateway, err := c.daemon.findGateway(gateway.URI().String())
				require.NoError(t, err)
				require.Equal(t, gateway.URI(), foundGateway.URI())
			},
		},
		{
			name:                 "SetGatewayLocalPort doesn't close or modify previous gateway if new port is occupied",
			gatewayNamesToCreate: []string{"gateway"},
			tcpPortAllocator:     &gatewaytest.MockTCPPortAllocator{PortsInUse: []string{"12345"}},
			testFunc: func(c *gatewayCRUDTestContext) {
				gateway := c.nameToGateway["gateway"]
				gatewayAddress := net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort())
				serveErr := make(chan error)
				go func() {
					err := gateway.Serve()
					serveErr <- err
				}()
				t.Cleanup(func() {
					gateway.Close()
					require.NoError(t, <-serveErr, "Gateway %s returned error from Serve()", gateway.URI())
				})

				_, err := c.daemon.SetGatewayLocalPort(context.Background(), gateway.URI().String(), "12345")
				require.ErrorContains(t, err, "address already in use")

				// Verify that the gateway still accepts connections on the old address.
				gatewaytest.BlockUntilGatewayAcceptsConnections(t, gatewayAddress)
			},
		},
		{
			name:                 "SetGatewayLocalPort is a noop if new port is equal to old port",
			gatewayNamesToCreate: []string{"gateway"},
			testFunc: func(c *gatewayCRUDTestContext) {
				gateway := c.nameToGateway["gateway"]
				localPort := gateway.LocalPort()
				require.Equal(t, 1, c.mockTCPPortAllocator.CallCount)

				_, err := c.daemon.SetGatewayLocalPort(context.Background(), gateway.URI().String(), localPort)
				require.NoError(t, err)

				require.Equal(t, 1, c.mockTCPPortAllocator.CallCount)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.tcpPortAllocator == nil {
				tt.tcpPortAllocator = &gatewaytest.MockTCPPortAllocator{}
			}

			homeDir := t.TempDir()
			mockGatewayCreator := &mockGatewayCreator{t: t}

			storage, err := clusters.NewStorage(clusters.Config{
				Dir:                homeDir,
				InsecureSkipVerify: true,
			})
			require.NoError(t, err)

			daemon, err := New(Config{
				Storage:          storage,
				GatewayCreator:   mockGatewayCreator,
				TCPPortAllocator: tt.tcpPortAllocator,
			})
			require.NoError(t, err)

			nameToGateway := make(map[string]*gateway.Gateway, len(tt.gatewayNamesToCreate))

			for _, gatewayName := range tt.gatewayNamesToCreate {
				gatewayName := gatewayName
				gateway, err := daemon.CreateGateway(context.Background(), CreateGatewayParams{
					TargetURI:             uri.NewClusterURI("foo").AppendDB(gatewayName).String(),
					TargetUser:            "alice",
					TargetSubresourceName: "",
					LocalPort:             "",
				})
				require.NoError(t, err)

				nameToGateway[gatewayName] = gateway
			}

			tt.testFunc(&gatewayCRUDTestContext{
				t:                    t,
				nameToGateway:        nameToGateway,
				mockGatewayCreator:   mockGatewayCreator,
				mockTCPPortAllocator: tt.tcpPortAllocator,
				daemon:               daemon,
			})
		})
	}
}
