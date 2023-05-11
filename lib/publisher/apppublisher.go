package publisher

type RemoteAppType string

const (
	Webapp     RemoteAppType = "REMOTE_WEB_APP"
	Server     RemoteAppType = "REMOTE_SERVER"
	Database   RemoteAppType = "REMOTE_DATABASE"
	Desktop    RemoteAppType = "REMOTE_DESKTOP"
	Kubernetes RemoteAppType = "REMOTE_KUBERNETES"

	//default sqs queue name
	defaultSQSQueueName = "remote-access-resource-change-notification-queue"
)

type AppChangeEvent struct {
	AppType RemoteAppType `json:"appType,omitempty"`
	Tenant  string        `json:"tenant,omitempty"`
}

// AppPublisher app publisher to publish the app changes
type AppPublisher interface {
	Publish(event AppChangeEvent) error
}

type AppPublisherConfig struct {
	SQSQueueName string
}

func (cfg *AppPublisherConfig) CheckAndSetDefaults() error {
	if cfg.SQSQueueName == "" {
		cfg.SQSQueueName = defaultSQSQueueName
	}
	return nil
}

func NewAppPublisher(config AppPublisherConfig) AppPublisher {
	config.CheckAndSetDefaults()
	return NewSQSAppPublisherService(config.SQSQueueName)
}
