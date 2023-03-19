// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package local

import (
	"context"

	"github.com/cloudflare/cfssl/log"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/encryption"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// CA is local implementation of Trust service that
// is using local backend
type CA struct {
	backend.Backend
	encryption.ItemEncrypter
}

// NewCAService returns new instance of CAService
func NewCAService(b backend.Backend) *CA {
	return &CA{
		Backend:       b,
		ItemEncrypter: encryption.NewItemEncryptionService(nil),
	}
}

func NewIdemeumCAService(b backend.Backend, es encryption.EncryptionService) *CA {
	log.Infof("Initializing CA service with encryption service : %v", es)
	return &CA{
		Backend:       b,
		ItemEncrypter: encryption.NewItemEncryptionService(es),
	}
}

// DeleteAllCertAuthorities deletes all certificate authorities of a certain type
func (s *CA) DeleteAllCertAuthorities(caType types.CertAuthType) error {
	startKey := backend.Key(authoritiesPrefix, string(caType))
	return s.DeleteRange(context.TODO(), startKey, backend.RangeEnd(startKey))
}

// CreateCertAuthority updates or inserts a new certificate authority
func (s *CA) CreateCertAuthority(ca types.CertAuthority) error {
	if err := services.ValidateCertAuthority(ca); err != nil {
		return trace.Wrap(err)
	}
	value, err := services.MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(authoritiesPrefix, string(ca.GetType()), ca.GetName()),
		Value:   value,
		Expires: ca.Expiry(),
	}

	encryptedItem, err := s.Encrypt(&item)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Create(context.TODO(), *encryptedItem)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists("cluster %q already exists", ca.GetName())
		}
		return trace.Wrap(err)
	}
	return nil
}

// UpsertCertAuthority updates or inserts a new certificate authority
func (s *CA) UpsertCertAuthority(ca types.CertAuthority) error {
	if err := services.ValidateCertAuthority(ca); err != nil {
		return trace.Wrap(err)
	}

	// try to skip writes that would have no effect
	if existing, err := s.GetCertAuthority(context.TODO(), types.CertAuthID{
		Type:       ca.GetType(),
		DomainName: ca.GetClusterName(),
	}, true); err == nil {
		if services.CertAuthoritiesEquivalent(existing, ca) {
			return nil
		}
	}

	value, err := services.MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(authoritiesPrefix, string(ca.GetType()), ca.GetName()),
		Value:   value,
		Expires: ca.Expiry(),
		ID:      ca.GetResourceID(),
	}

	encryptedItem, err := s.Encrypt(&item)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(context.TODO(), *encryptedItem)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CompareAndSwapCertAuthority updates the cert authority value
// if the existing value matches expected parameter, returns nil if succeeds,
// trace.CompareFailed otherwise.
func (s *CA) CompareAndSwapCertAuthority(new, expected types.CertAuthority) error {
	if err := services.ValidateCertAuthority(new); err != nil {
		return trace.Wrap(err)
	}

	key := backend.Key(authoritiesPrefix, string(new.GetType()), new.GetName())

	actualItem, err := s.Get(context.TODO(), key)
	if err != nil {
		return trace.Wrap(err)
	}

	decryptedActualItem, err := s.Decrypt(actualItem)
	if err != nil {
		return trace.Wrap(err)
	}

	actual, err := services.UnmarshalCertAuthority(decryptedActualItem.Value)
	if err != nil {
		return trace.Wrap(err)
	}

	if !services.CertAuthoritiesEquivalent(actual, expected) {
		return trace.CompareFailed("cluster %v settings have been updated, try again", new.GetName())
	}

	newValue, err := services.MarshalCertAuthority(new)
	if err != nil {
		return trace.Wrap(err)
	}

	newItem := backend.Item{
		Key:     key,
		Value:   newValue,
		Expires: new.Expiry(),
	}

	encryptedItem, err := s.Encrypt(&newItem)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s.CompareAndSwap(context.TODO(), *actualItem, *encryptedItem)
	if err != nil {
		if trace.IsCompareFailed(err) {
			return trace.CompareFailed("cluster %v settings have been updated, try again", new.GetName())
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteCertAuthority deletes particular certificate authority
func (s *CA) DeleteCertAuthority(id types.CertAuthID) error {
	if err := id.Check(); err != nil {
		return trace.Wrap(err)
	}
	// when removing a types.CertAuthority also remove any deactivated
	// types.CertAuthority as well if they exist.
	err := s.Delete(context.TODO(), backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	err = s.Delete(context.TODO(), backend.Key(authoritiesPrefix, string(id.Type), id.DomainName))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ActivateCertAuthority moves a CertAuthority from the deactivated list to
// the normal list.
func (s *CA) ActivateCertAuthority(id types.CertAuthID) error {
	item, err := s.Get(context.TODO(), backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.BadParameter("can not activate cert authority %q which has not been deactivated", id.DomainName)
		}
		return trace.Wrap(err)
	}

	decryptedItem, err := s.Decrypt(item)
	if err != nil {
		return trace.Wrap(err)
	}

	certAuthority, err := services.UnmarshalCertAuthority(
		decryptedItem.Value, services.WithResourceID(decryptedItem.ID), services.WithExpires(decryptedItem.Expires))
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertCertAuthority(certAuthority)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.Delete(context.TODO(), backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName))
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeactivateCertAuthority moves a CertAuthority from the normal list to
// the deactivated list.
func (s *CA) DeactivateCertAuthority(id types.CertAuthID) error {
	certAuthority, err := s.GetCertAuthority(context.TODO(), id, true)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("can not deactivate cert authority %q which does not exist", id.DomainName)
		}
		return trace.Wrap(err)
	}

	err = s.DeleteCertAuthority(id)
	if err != nil {
		return trace.Wrap(err)
	}

	value, err := services.MarshalCertAuthority(certAuthority)
	if err != nil {
		return trace.Wrap(err)
	}

	item := backend.Item{
		Key:     backend.Key(authoritiesPrefix, deactivatedPrefix, string(id.Type), id.DomainName),
		Value:   value,
		Expires: certAuthority.Expiry(),
		ID:      certAuthority.GetResourceID(),
	}

	encryptedItem, err := s.Encrypt(&item)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.Put(context.TODO(), *encryptedItem)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (s *CA) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error) {
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := s.Get(ctx, backend.Key(authoritiesPrefix, string(id.Type), id.DomainName))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	decryptedItem, err := s.Decrypt(item)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ca, err := services.UnmarshalCertAuthority(
		decryptedItem.Value, services.AddOptions(opts, services.WithResourceID(decryptedItem.ID), services.WithExpires(decryptedItem.Expires))...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := services.ValidateCertAuthority(ca); err != nil {
		return nil, trace.Wrap(err)
	}
	setSigningKeys(ca, loadSigningKeys)
	return ca, nil
}

func setSigningKeys(ca types.CertAuthority, loadSigningKeys bool) {
	if loadSigningKeys {
		return
	}
	types.RemoveCASecrets(ca)
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (s *CA) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadSigningKeys bool, opts ...services.MarshalOption) ([]types.CertAuthority, error) {
	if err := caType.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Get all items in the bucket.
	startKey := backend.Key(authoritiesPrefix, string(caType))
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Marshal values into a []types.CertAuthority slice.
	cas := make([]types.CertAuthority, len(result.Items))
	for i, item := range result.Items {
		decryptedItem, err := s.Decrypt(&item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ca, err := services.UnmarshalCertAuthority(
			decryptedItem.Value, services.AddOptions(opts,
				services.WithResourceID(decryptedItem.ID),
				services.WithExpires(decryptedItem.Expires))...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := services.ValidateCertAuthority(ca); err != nil {
			return nil, trace.Wrap(err)
		}
		setSigningKeys(ca, loadSigningKeys)
		cas[i] = ca
	}

	return cas, nil
}

const (
	authoritiesPrefix = "authorities"
	deactivatedPrefix = "deactivated"
)
