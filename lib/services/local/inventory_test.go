/*
Copyright 2022 Gravitational, Inc.

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

package local

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestInstanceUpsert verifies basic expected behavior of instance creation/update.
func TestInstanceUpsert(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	defer backend.Close()

	presence := NewPresenceService(backend)

	instance1, err := types.NewInstance(uuid.NewString(), types.InstanceSpecV1{
		Hostname: "h1",
	})
	require.NoError(t, err)

	err = presence.UpsertInstance(ctx, instance1)
	require.NoError(t, err)

	// get the inserted instance
	instances, err := stream.Collect(presence.GetInstances(ctx, types.InstanceFilter{}))
	require.NoError(t, err)
	require.Len(t, instances, 1)

	require.Equal(t, "h1", instances[0].GetHostname())

	// verify that expiry and last_seen are automatically set to expected values.
	exp1 := instances[0].Expiry()
	seen1 := instances[0].GetLastSeen()
	require.False(t, exp1.IsZero())
	require.False(t, seen1.IsZero())
	require.Equal(t, presence.Clock().Now().UTC(), seen1)
	require.Equal(t, seen1.Add(apidefaults.ServerAnnounceTTL), exp1)

	require.True(t, exp1.After(presence.Clock().Now()))
	require.False(t, exp1.After(presence.Clock().Now().Add(apidefaults.ServerAnnounceTTL*2)))

	instance2, err := types.NewInstance(instance1.GetName(), types.InstanceSpecV1{
		Hostname: "h2",
	})
	require.NoError(t, err)

	err = presence.UpsertInstance(ctx, instance2)
	require.NoError(t, err)

	// load new instance state
	instances2, err := stream.Collect(presence.GetInstances(ctx, types.InstanceFilter{}))
	require.NoError(t, err)
	require.Len(t, instances2, 1)

	// ensure that updated state propagated
	require.Equal(t, "h2", instances2[0].GetHostname())
}

// TestInstanceFiltering tests basic filtering options. A sufficiently large
// instance count is used to ensure that queries span many pages.
func TestInstanceFiltering(t *testing.T) {
	const count = 10_000
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// NOTE: backend must be memory, since parallel subtests are used (makes correct cleanup of
	// filesystem state tricky).
	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	defer backend.Close()

	presence := NewPresenceService(backend)

	// store an odd and an even uuid for later use in queries
	var evenID, oddID string

	evenServices := []types.SystemRole{"even"}
	oddServices := []types.SystemRole{"odd"}

	evenVersion := "v2.4.6"
	oddVersion := "v3.5.7"

	allServices := append(evenServices, oddServices...)

	// create a bunch of instances with an even mix of odd/even "services".
	for i := 0; i < count; i++ {
		serverID := uuid.NewString()
		var services []types.SystemRole
		var version string
		if i%2 == 0 {
			services = evenServices
			version = evenVersion
			evenID = serverID
		} else {
			services = oddServices
			version = oddVersion
			oddID = serverID
		}

		instance, err := types.NewInstance(serverID, types.InstanceSpecV1{
			Services: services,
			Version:  version,
		})
		require.NoError(t, err)

		err = presence.UpsertInstance(ctx, instance)
		require.NoError(t, err)
	}

	// check a few simple queries
	tts := []struct {
		filter    types.InstanceFilter
		even, odd int
		desc      string
	}{
		{
			filter: types.InstanceFilter{
				Services: evenServices,
			},
			even: count / 2,
			desc: "all even services",
		},
		{
			filter: types.InstanceFilter{
				ServerID: oddID,
			},
			odd:  1,
			desc: "single-instance direct",
		},
		{
			filter: types.InstanceFilter{
				ServerID: evenID,
				Services: oddServices,
			},
			desc: "non-matching id+service pair",
		},
		{
			filter: types.InstanceFilter{
				ServerID: evenID,
				Services: evenServices,
			},
			even: 1,
			desc: "matching id+service pair",
		},
		{
			filter: types.InstanceFilter{
				Services: allServices,
			},
			even: count / 2,
			odd:  count / 2,
			desc: "all services",
		},
		{
			filter: types.InstanceFilter{
				Version: evenVersion,
			},
			even: count / 2,
			desc: "single version",
		},
	}

	for _, testCase := range tts {
		tt := testCase
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			// load instances with given filter
			instances, err := stream.Collect(presence.GetInstances(ctx, tt.filter))
			require.NoError(t, err)

			// aggregate number of s
			var even, odd int
			for _, instance := range instances {
				require.Len(t, instance.GetServices(), 1)
				switch service := instance.GetServices()[0]; service {
				case "even":
					even++
				case "odd":
					odd++
				default:
					t.Fatalf("Unexpected service: %+v", service)
				}
			}

			require.Equal(t, tt.even, even)
			require.Equal(t, tt.odd, odd)
		})
	}
}
