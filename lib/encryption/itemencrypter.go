package encryption

import (
	"encoding/json"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
		encryptedValue, err := s.es.Encrypt(item.Value)
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
		return nil, trace.Wrap(err)
	}

	if encryptedValue.Nonce == nil {
		return item, nil
	}

	if s.es == nil {
		return nil, trace.BadParameter("item encrypter service missing encryption service")
	}

	value, err := s.es.Decrypt(encryptedValue)
	if err != nil {
		return nil, err
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

func ToEncryptedValue(data []byte) (EncryptedValue, error) {
	if !json.Valid(data) {
		return EncryptedValue{Value: data}, nil
	}

	encryptedValue := EncryptedValue{}
	err := json.Unmarshal(data, &encryptedValue)
	if err != nil {
		return EncryptedValue{}, trace.Wrap(err)
	}
	return encryptedValue, nil
}
