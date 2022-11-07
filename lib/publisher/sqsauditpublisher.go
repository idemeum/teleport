package publisher

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

type sqsAuditMessagePublisherService struct {
	sqsService *sqs.SQS
	cfg        AuditPublisherConfig
}

func NewAuditMessagePublisherService(cfg AuditPublisherConfig) AuditMessagePublisher {
	log.Info("Initializing the sqs audit publisher service")
	session := session.Must(session.NewSession(&aws.Config{
		Region: &cfg.Region,
	}))

	sqsService := sqs.New(session)
	log.Info("Initialized the sqs audit publisher service")
	return &sqsAuditMessagePublisherService{sqsService, cfg}
}

func (s *sqsAuditMessagePublisherService) Publish(audit AuditMessage) error {
	queueUrlOutput, err := s.sqsService.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &s.cfg.SQSQueueName,
	})
	if err != nil {
		log.Errorf("Failed to retrieve the qeueue url:%v err: %v", s.cfg.SQSQueueName, err)
		return trace.Wrap(err)
	}

	output, err := s.sqsService.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(audit.MessageBody),
		QueueUrl:    queueUrlOutput.QueueUrl,
	})

	if err != nil {
		log.Debugf("Failed to publish the message err :%v", err)
		return trace.Wrap(err)
	}

	log.Debugf("Message published with messageId :%v", output.MessageId)
	return nil
}
