package publisher

import log "github.com/sirupsen/logrus"

type RemoteAppType string

const (
	Webapp     RemoteAppType = "REMOTE_WEB_APP"
	Server     RemoteAppType = "REMOTE_SERVER"
	Database   RemoteAppType = "REMOTE_DATABASE"
	Desktop    RemoteAppType = "REMOTE_DESKTOP"
	Kubernetes RemoteAppType = "REMOTE_KUBERNETES"
	Invalid    RemoteAppType = "INVALID"

	//default sqs queue name
	defaultSQSQueueName = "remote-access-resource-change-notification-queue"

	// added delay in seconds for the sqs message
	// teleport uses in-memory cache for resources when the resource is added in database
	// it takes time to reflect in local in-memory cache.
	// Added delay for sqs message so that when the app management retrives the resources
	// they are reflected
	defaultDelayInSeconds = 30
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
	TenantUrl      string
	SQSQueueName   string
	Enabled        bool
	DelayInSeconds int64
}

func (cfg *AppPublisherConfig) CheckAndSetDefaults() error {
	if cfg.SQSQueueName == "" {
		cfg.SQSQueueName = defaultSQSQueueName
	}

	if cfg.DelayInSeconds == 0 {
		cfg.DelayInSeconds = defaultDelayInSeconds
	}
	return nil
}

func NewAppPublisher(config AppPublisherConfig) AppPublisher {
	config.CheckAndSetDefaults()
	if config.Enabled {
		log.Info("Publishing app changes to idemeum enabled")
		return &defaultAppPublisher{
			publisher: NewSQSAppPublisherService(config),
			cfg:       config,
		}
	}
	log.Info("Publishing app changes to idemeum disabled")
	return &defaultAppPublisher{publisher: &noOpAppPublisher{}, cfg: config}
}

type defaultAppPublisher struct {
	publisher AppPublisher
	cfg       AppPublisherConfig
}

func (p *defaultAppPublisher) Publish(event AppChangeEvent) error {
	if event.Tenant == "" {
		event.Tenant = p.cfg.TenantUrl
	}

	log.Infof("Publishing event for tenant: %v and app type: %v", event.Tenant, event.AppType)
	return p.publisher.Publish(event)
}

type noOpAppPublisher struct {
}

func (p *noOpAppPublisher) Publish(event AppChangeEvent) error {
	log.Infof("No op publishing event for tenant: %v and app type: %v", event.Tenant, event.AppType)
	return nil
}
