package encryption

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

type EncryptionService interface {
	Encrypt(text []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
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
		log.Info("encryption service config cluster name is missing")
		return trace.BadParameter("encryption service missing cluster name")
	}

	if cfg.Region == "" {
		log.Info("encryption service config region is missing")
		return trace.BadParameter("encryption service missing region")
	}

	if cfg.KmsKeyId == "" {
		log.Info("encryption service config KmsKeyId is missing")
		return trace.BadParameter("encryption service missing KmsKeyId")
	}
	return nil
}

func NewEncryptionService(config KMSEncryptionConfig) EncryptionService {
	config.CheckAndSetDefaults()
	if config.Enabled {
		log.Info("Encryption service enabled")
		return newKMSEncryptionService(config)
	}

	log.Info("Encryption service not enabled")
	return nil
}

type kmsEncryptionService struct {
	kmsService *kms.KMS
	cfg        KMSEncryptionConfig
}

func newKMSEncryptionService(cfg KMSEncryptionConfig) EncryptionService {
	log.Info("Initializing the kms server")
	session := session.Must(session.NewSession(&aws.Config{
		Region: &cfg.Region,
	}))

	kmsService := kms.New(session)
	log.Info("Initialized the sqs app publisher service")
	return &kmsEncryptionService{kmsService, cfg}
}

func (s *kmsEncryptionService) Encrypt(text []byte) ([]byte, error) {
	result, err := s.kmsService.Encrypt(&kms.EncryptInput{
		KeyId:             aws.String(s.cfg.KmsKeyId),
		Plaintext:         []byte(text),
		EncryptionContext: getEncryptionContext(s.cfg.ClusterName),
	})

	return result.CiphertextBlob, err
}

// Decrypt implements EncryptionService
func (s *kmsEncryptionService) Decrypt(data []byte) ([]byte, error) {
	result, err := s.kmsService.Decrypt(&kms.DecryptInput{
		KeyId:             aws.String(s.cfg.KmsKeyId),
		CiphertextBlob:    []byte(data),
		EncryptionContext: getEncryptionContext(s.cfg.ClusterName),
	})

	return result.Plaintext, err
}

type TestEncryptionService struct {
}

// Encrypt implements EncryptionService
func (*TestEncryptionService) Encrypt(data []byte) ([]byte, error) {
	log.Info("encrypting")
	encrypted := string("test") + string(data)
	return []byte(encrypted), nil
}

// Decrypt implements EncryptionService
func (*TestEncryptionService) Decrypt(data []byte) ([]byte, error) {
	log.Info("decrypting")
	//remove "test"
	return data[4:], nil
}

func getEncryptionContext(ClusterName string) map[string]*string {
	encryptionContext := make(map[string]*string)
	encryptionContext["clusterName"] = aws.String(ClusterName)
	return encryptionContext
}
