package publisher

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

type sqsAppPublisherService struct {
	sqsService *sqs.SQS
	config     sqsPublisherConfig
}

type sqsPublisherConfig struct {
	QueueName string
}

func NewSQSAppPublisherService(queueName string) AppPublisher {
	log.Info("Initializing the sqs app publisher service")
	session := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"),
	}))

	sqsService := sqs.New(session)
	config := sqsPublisherConfig{
		QueueName: queueName,
	}
	log.Info("Initialized the sqs app publisher service")
	return &sqsAppPublisherService{sqsService, config}
}

func (s *sqsAppPublisherService) Publish(event AppChangeEvent) error {
	queueUrlOutput, err := s.sqsService.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: &s.config.QueueName,
	})
	if err != nil {
		log.Errorf("Failed to retrieve the qeueue url:%v err: %v", s.config.QueueName, err)
		return trace.Wrap(err)
	}
	messageJsonData, err := json.Marshal(event)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("Publishing the message for app type: %v", event.AppType)
	_, err = s.sqsService.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(string(messageJsonData)),
		QueueUrl:    queueUrlOutput.QueueUrl,
	})

	if err != nil {
		log.Errorf("Failed to publish the message for app type: %v  err : %v", event.AppType, err)
		return err
	}
	log.Infof("Sent message for app type: %v successfully", event.AppType)
	return nil
}
