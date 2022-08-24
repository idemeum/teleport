package publisher

import (
	"context"
	"encoding/json"

	"github.com/cloudflare/cfssl/log"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
)

const (
	//default sqs queue name
	defaultAuditSQSQueueName = "remote-access-audit-event-notification-queue"
)

type AuditPublisher interface {
	apievents.Emitter
}

type AuditPublisherConfig struct {
	Enabled      bool
	SQSQueueName string
	EventByTypes map[string]string
}

type AuditPublisherService struct {
	publisher AuditMessagePublisher
	cfg       AuditPublisherConfig
}

type AuditMessage struct {
	MessageBody string
}

// AuditMessagePublisher responsible for publish the audit message to destination
type AuditMessagePublisher interface {
	Publish(audit AuditMessage) error
}

func (cfg *AuditPublisherConfig) CheckAndSetDefaults() error {
	if cfg.SQSQueueName == "" {
		cfg.SQSQueueName = defaultAuditSQSQueueName
	}

	if len(cfg.EventByTypes) == 0 {
		cfg.EventByTypes = getDefaultEventByTypes()
	}
	return nil
}

func NewAuditPublisher(cfg AuditPublisherConfig) AuditPublisher {
	if cfg.Enabled {
		log.Info("Audit publishing is eanbled")
		cfg.CheckAndSetDefaults()
		return &AuditPublisherService{
			publisher: NewAuditMessagePublisherService(cfg),
			cfg:       cfg,
		}
	}
	log.Info("Audit publishing is disabled")
	return &noOpAuditPublisher{}
}

// EmitAuditEvent implements AuditPublisher
func (s *AuditPublisherService) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	//filter the required event to publish
	if _, ok := s.cfg.EventByTypes[event.GetType()]; !ok {
		log.Debugf("Skipping the audit event id:%v type: %v", event.GetID(), event.GetType())
		return nil
	}

	log.Debugf("Publishing the audit event [ id %v  cluster: %v and type: %v ]", event.GetID(), event.GetClusterName(), event.GetType())
	messageBody, err := json.Marshal(event)
	if err != nil {
		log.Errorf("Failed to serialize the audit event to json err: %v", err)
		return trace.Wrap(err)
	}

	err = s.publisher.Publish(AuditMessage{
		MessageBody: string(messageBody),
	})

	if err != nil {
		log.Debugf("Failed to publish the audit event [ id %v  cluster: %v and type: %v ]", event.GetID(), event.GetClusterName(), event.GetType())
		return err
	}

	log.Debugf("Published the audit event [ id %v  cluster: %v and type: %v ]", event.GetID(), event.GetClusterName(), event.GetType())
	return nil
}

func getDefaultEventByTypes() map[string]string {
	auditTypes := make(map[string]string)
	auditTypes[events.SessionStartEvent] = events.SessionStartEvent
	auditTypes[events.SessionEndEvent] = events.SessionEndEvent
	auditTypes[events.AppSessionStartEvent] = events.AppSessionStartEvent
	return auditTypes
}

type noOpAuditPublisher struct {
}

// EmitAuditEvent implements AuditPublisher
func (*noOpAuditPublisher) EmitAuditEvent(context.Context, apievents.AuditEvent) error {
	return nil
}
