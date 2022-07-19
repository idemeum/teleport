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

package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gatewaytest"

	"github.com/stretchr/testify/require"
)

func TestCLICommandUsesCLICommandProvider(t *testing.T) {
	gateway := Gateway{
		cfg: &Config{
			TargetName:            "foo",
			TargetSubresourceName: "bar",
			Protocol:              defaults.ProtocolPostgres,
			CLICommandProvider:    mockCLICommandProvider{},
			TCPPortAllocator:      &gatewaytest.MockTCPPortAllocator{},
		},
	}

	command, err := gateway.CLICommand()
	require.NoError(t, err)

	require.Equal(t, "foo/bar", command)
}

func TestGatewayStart(t *testing.T) {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	t.Cleanup(func() {
		hs.Close()
	})

	gateway, err := New(
		Config{
			TargetName:         "foo",
			TargetURI:          uri.NewClusterURI("bar").AppendDB("foo").String(),
			TargetUser:         "alice",
			Protocol:           defaults.ProtocolPostgres,
			CertPath:           "../../../fixtures/certs/proxy1.pem",
			KeyPath:            "../../../fixtures/certs/proxy1-key.pem",
			Insecure:           true,
			WebProxyAddr:       hs.Listener.Addr().String(),
			CLICommandProvider: mockCLICommandProvider{},
			TCPPortAllocator:   &gatewaytest.MockTCPPortAllocator{},
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() { gateway.Close() })
	gatewayAddress := net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort())

	require.NotEmpty(t, gateway.LocalPort())
	require.NotEqual(t, "0", gateway.LocalPort())

	serveErr := make(chan error)

	go func() {
		err := gateway.Serve()
		serveErr <- err
	}()

	gatewaytest.BlockUntilGatewayAcceptsConnections(t, gatewayAddress)

	err = gateway.Close()
	require.NoError(t, err)
	require.NoError(t, <-serveErr)
}

func TestNewWithLocalPortStartsListenerOnNewPortIfPortIsFree(t *testing.T) {
	tcpPortAllocator := gatewaytest.MockTCPPortAllocator{}
	oldGateway := serveGateway(t, &tcpPortAllocator)
	originalCloseContext := oldGateway.closeContext

	gateway, err := NewWithLocalPort(*oldGateway, "12345")
	require.NoError(t, err)

	require.Equal(t, "12345", gateway.LocalPort())

	// Verify that the gateway is accepting connections on the new listener.
	//
	// MockTCPPortAllocator.Listen returns a listener which occupies a random port but its Addr method
	// reports the port that was passed to Listen. In order to actually dial the gateway we need to
	// obtain the real address from the listener.
	newGatewayAddress := tcpPortAllocator.RecentListener().RealAddr().String()
	gatewaytest.BlockUntilGatewayAcceptsConnections(t, newGatewayAddress)

	// Verify that the old context was canceled.
	//
	// What we really want to test is if the old listener was closed. Unfortunately, we don't seem to
	// have a straightforward way to test this as at this point another process might have started
	// listening on that port.
	require.ErrorIs(t, originalCloseContext.Err(), context.Canceled,
		"The listener on the old port wasn't closed after starting a listener on the new port.")
}

// TODO: Change this test to NewWithLocalPort.
func TestSetLocalPortAndRestartDoesntStopGatewayIfNewPortIsOccupied(t *testing.T) {
	tcpPortAllocator := gatewaytest.MockTCPPortAllocator{PortsInUse: []string{"12345"}}
	gateway := serveGateway(t, &tcpPortAllocator)
	originalPort := gateway.LocalPort()
	originalCloseContext := gateway.closeContext

	err := gateway.SetLocalPortAndRestart("12345")
	require.ErrorContains(t, err, "address already in use")
	require.Equal(t, originalPort, gateway.LocalPort())

	// Verify that we don't stop the gateway if we failed to start a listener on the specified port.
	require.NoError(t, originalCloseContext.Err(),
		"The listener on the current port was closed even though we failed to start a listener on the new port.")
}

func TestSetLocalPortAndRestartIsNoopIfNewPortEqualsOldPort(t *testing.T) {
	tcpPortAllocator := gatewaytest.MockTCPPortAllocator{}
	gateway := serveGateway(t, &tcpPortAllocator)
	port := gateway.LocalPort()
	gatewayAddress := tcpPortAllocator.RecentListener().RealAddr().String()
	originalCloseContext := gateway.closeContext

	err := gateway.SetLocalPortAndRestart(port)
	require.NoError(t, err)

	// Verify that we don't stop the gateway if the new port is equal to the old port.
	require.NoError(t, originalCloseContext.Err(),
		"The listener on the current port was closed even though the new port is equal to the old port.")
	gatewaytest.BlockUntilGatewayAcceptsConnections(t, gatewayAddress)
}

type mockCLICommandProvider struct{}

func (m mockCLICommandProvider) GetCommand(gateway *Gateway) (string, error) {
	command := fmt.Sprintf("%s/%s", gateway.TargetName(), gateway.TargetSubresourceName())
	return command, nil
}

// serveGateway starts a gateway and blocks until it accepts connections.
func serveGateway(t *testing.T, tcpPortAllocator TCPPortAllocator) *Gateway {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	t.Cleanup(func() {
		hs.Close()
	})

	gateway, err := New(
		Config{
			TargetName:         "foo",
			TargetURI:          uri.NewClusterURI("bar").AppendDB("foo").String(),
			TargetUser:         "alice",
			Protocol:           defaults.ProtocolPostgres,
			CertPath:           "../../../fixtures/certs/proxy1.pem",
			KeyPath:            "../../../fixtures/certs/proxy1-key.pem",
			Insecure:           true,
			WebProxyAddr:       hs.Listener.Addr().String(),
			CLICommandProvider: mockCLICommandProvider{},
			TCPPortAllocator:   tcpPortAllocator,
		},
	)
	require.NoError(t, err)

	serveErr := make(chan error)
	go func() {
		err := gateway.Serve()
		serveErr <- err
	}()
	t.Cleanup(func() {
		gateway.Close()
		require.NoError(t, <-serveErr, "Gateway %s returned error from Serve()", gateway.URI())
	})

	gatewayAddress := net.JoinHostPort(gateway.LocalAddress(), gateway.LocalPort())
	gatewaytest.BlockUntilGatewayAcceptsConnections(t, gatewayAddress)

	return gateway
}
