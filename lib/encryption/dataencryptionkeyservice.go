package encryption

import (
	"context"
	"crypto/rand"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	keyAlgorithm            = "AES_256"
	dataEncryptionKeyPrefix = "dataencryptionkey"
)

type DataEncryptionKeyService interface {
	Init() error
	getKey() ([]byte, error)
}

type KMSEncryptionConfig struct {
	ClusterName string
	Region      string
	KmsKeyId    string
	Enabled     bool
}

func (cfg *KMSEncryptionConfig) CheckAndSetDefaults() error {
	if !cfg.Enabled {
		return nil
	}

	if cfg.ClusterName == "" {
		log.Info("Data Encryption key service config cluster name is missing")
		return trace.BadParameter("data Encryption key service missing cluster name")
	}

	if cfg.Region == "" {
		log.Info("Data Encryption key service config region is missing")
		return trace.BadParameter("data Encryption key service missing region")
	}

	if cfg.KmsKeyId == "" {
		log.Info("Data Encryption key service config KmsKeyId is missing")
		return trace.BadParameter("data Encryption key service missing KmsKeyId")
	}
	return nil
}

func NewDataEncryptionKeyService(config KMSEncryptionConfig, backend backend.Backend) DataEncryptionKeyService {
	config.CheckAndSetDefaults()
	if config.Enabled {
		log.Infof("Data Encryption key service enabled for cluster: %v", config.ClusterName)
		return newDataEncryptionKeyService(config, backend)
	}

	log.Info("Data Encryption key service not enabled")
	return nil
}

type kmsDataEncryptionKeyService struct {
	kmsService   *kms.KMS
	cfg          KMSEncryptionConfig
	backend      backend.Backend
	decryptedKey *kms.DecryptOutput
}

func newDataEncryptionKeyService(cfg KMSEncryptionConfig, backend backend.Backend) DataEncryptionKeyService {
	log.Info("Initializing the kms service")
	session := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(cfg.Region),
	}))

	kmsService := kms.New(session)
	log.Info("Initialized the kms service")
	return &kmsDataEncryptionKeyService{kmsService: kmsService,
		cfg:     cfg,
		backend: backend,
	}
}

func (s *kmsDataEncryptionKeyService) getKey() ([]byte, error) {
	if s.decryptedKey == nil {
		return nil, trace.Errorf("data encryption not initialized")
	}
	return s.decryptedKey.Plaintext, nil
}

func (s *kmsDataEncryptionKeyService) Init() error {
	log.Infof("Initializing the data encryption key")
	if s.decryptedKey != nil {
		return nil
	}

	item, err := s.backend.Get(context.TODO(), backend.Key(dataEncryptionKeyPrefix, s.cfg.ClusterName))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		log.Infof("Creating new data encryption key, as it does not exists")
		item, err = s.generateDataEncryptionKey()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	decryptedKeyResult, err := s.kmsService.Decrypt(&kms.DecryptInput{
		KeyId:             aws.String(s.cfg.KmsKeyId),
		CiphertextBlob:    []byte(item.Value),
		EncryptionContext: getEncryptionContext(s.cfg.ClusterName),
	})

	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Initialized the data encryption key")
	s.decryptedKey = decryptedKeyResult
	return nil
}

func (s *kmsDataEncryptionKeyService) generateDataEncryptionKey() (*backend.Item, error) {
	rsp, err := s.kmsService.GenerateDataKey(&kms.GenerateDataKeyInput{
		KeyId:             aws.String(s.cfg.KmsKeyId),
		KeySpec:           aws.String(keyAlgorithm),
		EncryptionContext: getEncryptionContext(s.cfg.ClusterName),
	})
	if err != nil {
		log.Error("Failed to generate data key", err)
		return nil, err
	}

	newItem := backend.Item{
		Key:   backend.Key(dataEncryptionKeyPrefix, s.cfg.ClusterName),
		Value: rsp.CiphertextBlob,
	}

	_, err = s.backend.Create(context.TODO(), newItem)
	if err != nil {
		log.Error("Failed to save data encryption key to database", err)
		return nil, err
	}

	return &newItem, nil
}

type TestDataEncryptionService struct {
	decryptedKey []byte
}

func (s *TestDataEncryptionService) getKey() ([]byte, error) {
	if s.decryptedKey == nil {
		return nil, trace.Errorf("data encryption not initialized")
	}
	return s.decryptedKey, nil
}

func (s *TestDataEncryptionService) Init() error {
	if s.decryptedKey == nil {
		s.decryptedKey = make([]byte, 32)
		_, err := rand.Read(s.decryptedKey)
		if err != nil {
			return err
		}
	}
	return nil
}

func getEncryptionContext(ClusterName string) map[string]*string {
	encryptionContext := make(map[string]*string)
	encryptionContext["clusterName"] = aws.String(ClusterName)
	return encryptionContext
}
