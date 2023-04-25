package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"io"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

type EncryptedValue struct {
	// value which is encrypted using the key
	Value []byte
	// Nonce used for every value encryption using the key
	Nonce []byte
}

type EncryptionService interface {
	Encrypt(text []byte) ([]byte, error)
	Decrypt(holder EncryptedValue) ([]byte, error)
}

func NewEncryptionService(dekService DataEncryptionKeyService) EncryptionService {
	return &kmsEncryptionService{dekService}
}

type kmsEncryptionService struct {
	dekService DataEncryptionKeyService
}

func (s *kmsEncryptionService) Encrypt(text []byte) ([]byte, error) {
	if s.dekService == nil {
		return text, nil
	}

	dataEncryptionKey, err := s.dekService.getKey()
	if err != nil {
		log.Error("Failed to get data encryption data key", err)
		return nil, err
	}

	cipherText, nonce, err := aesGcmEncrypt(dataEncryptionKey, text)
	if err != nil {
		log.Error("Failed to encrypt data using data encryption key", err)
		return nil, err
	}
	// Initialize payload
	p := EncryptedValue{
		Value: cipherText,
		Nonce: nonce,
	}

	log.Debug("encrypted data successfully")
	return json.Marshal(p)
}

// Decrypt implements EncryptionService
func (s *kmsEncryptionService) Decrypt(p EncryptedValue) ([]byte, error) {
	if p.Nonce == nil {
		return p.Value, nil
	}

	if s.dekService == nil {
		log.Errorf("data encryption key service not configured, failing to decrypt the data")
		return nil, trace.Errorf("data encryption key service not configured")
	}

	dataEncryptionKey, err := s.dekService.getKey()
	if err != nil {
		log.Error("Failed to get data encryption data key", err)
		return nil, err
	}

	log.Debug("decrypting the data")
	return aesGcmDecrypt(dataEncryptionKey, p.Value, p.Nonce)
}

// aesGcmEncrypt takes an encryption key and a plaintext and encrypts it with AES256 in GCM mode,
// which provides authenticated encryption. Returns the ciphertext and the used nonce.
func aesGcmEncrypt(key, plaintextBytes []byte) (ciphertext, nonce []byte, err error) {

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	// Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	nonce = make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	ciphertext = aesgcm.Seal(nil, nonce, plaintextBytes, nil)
	return ciphertext, nonce, nil
}

// aesGcmDecrypt takes an decryption key, a ciphertext
// and the corresponding nonce and decrypts it with AES256 in GCM mode. Returns the plaintext string.
func aesGcmDecrypt(key, ciphertext, nonce []byte) (plaintext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintextBytes, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintextBytes, nil
}
