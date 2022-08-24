package publisher

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/events"
	"github.com/stretchr/testify/require"
)

func TestAuditPublisher(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()

	events := events.GenerateTestSession(events.SessionParams{PrintEvents: 0, ClusterName: "example.remote.idemeumlab.com"})
	emitter := newAuditPublisher()

	for _, event := range events {
		err := emitter.EmitAuditEvent(ctx, event)
		require.NoError(t, err)
	}
}

func newAuditPublisher() AuditPublisher {
	return &AuditPublisherService{
		publisher: &noOpIdemeumAuditPublisher{},
		cfg:       AuditPublisherConfig{},
	}
}

type noOpIdemeumAuditPublisher struct {
}

func (*noOpIdemeumAuditPublisher) Publish(message AuditMessage) error {
	return nil
}
