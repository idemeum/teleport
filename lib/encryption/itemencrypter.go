package encryption

import (
	"encoding/json"

	"github.com/cloudflare/cfssl/log"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
)

type ItemEncrypter interface {
	Encrypt(item *backend.Item) (*backend.Item, error)
	Decrypt(item *backend.Item) (*backend.Item, error)
}

type itemEncrypter struct {
	es EncryptionService
}

func NewItemEncryptionService(es EncryptionService) ItemEncrypter {
	log.Infof("initializing item encryption service: %v", es)
	return &itemEncrypter{es: es}
}

func (s *itemEncrypter) Encrypt(item *backend.Item) (*backend.Item, error) {
	if s.es != nil {
		itemEncryptedValue, err := s.es.Encrypt(item.Value)
		if err != nil {
			return item, trace.Wrap(err)
		}
		encryptedValue, err := json.Marshal(EncryptedValue{
			Value:     itemEncryptedValue,
			Encrypted: true,
		})

		if err != nil {
			return item, trace.Wrap(err)
		}

		newItem := &backend.Item{
			Key:     item.Key,
			Value:   encryptedValue,
			Expires: item.Expires,
			ID:      item.ID,
			LeaseID: item.LeaseID,
		}
		return newItem, nil

	}
	return item, nil
}

func (s *itemEncrypter) Decrypt(item *backend.Item) (*backend.Item, error) {
	encryptedValue, err := ToEncryptedValue(item.Value)
	if err != nil {
		return item, trace.Wrap(err)
	}

	if !encryptedValue.Encrypted {
		return item, nil
	}

	if encryptedValue.Encrypted && s.es == nil {
		return item, trace.BadParameter("trust service missing encryption service")
	}

	value, err := s.es.Decrypt(encryptedValue.Value)
	if err != nil {
		return item, err
	}
	newItem := &backend.Item{
		Key:     item.Key,
		Value:   value,
		Expires: item.Expires,
		ID:      item.ID,
		LeaseID: item.LeaseID,
	}
	return newItem, nil
}

type EncryptedValue struct {
	Value     []byte
	Encrypted bool
}

func ToEncryptedValue(data []byte) (EncryptedValue, error) {
	if !json.Valid(data) {
		return EncryptedValue{Value: data, Encrypted: false}, nil
	}

	encryptedValue := EncryptedValue{}
	err := json.Unmarshal(data, &encryptedValue)
	if err != nil {
		return EncryptedValue{}, trace.Wrap(err)
	}
	return encryptedValue, nil
}
