package actions

import (
	"errors"
	"fmt"

	"github.com/equinor/radix-common/utils/slice"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Max size of secret data. Includes key name and value
	defaultMaxPayloadDataSize int = 1024 * 512 // 0.5MB
)

var (
	ErrPayloadTooLarge = errors.New("payload too large")
)

type payloadSecrets struct {
	secrets          []corev1.Secret
	maxDataSize      int
	secretNamePrefix string
	secretLabels     map[string]string
}

func (p *payloadSecrets) AddPayload(payload string) (secretName, key string, err error) {
	secret, err := p.getSecretWithCapacity(len(payload))
	if err != nil {
		return "", "", err
	}
	key = fmt.Sprintf("p%d", len(secret.StringData))
	secret.StringData[key] = payload
	return secret.Name, key, nil
}

func (p *payloadSecrets) getSecretWithCapacity(capacity int) (*corev1.Secret, error) {
	if capacity > int(p.getMaxPayloadDataSize()) {
		return nil, ErrPayloadTooLarge
	}
	idx := slice.FindIndex(p.secrets, func(s corev1.Secret) bool {
		var usedCapacity int
		for _, d := range s.StringData {
			usedCapacity += len(d)
		}
		return p.getMaxPayloadDataSize()-usedCapacity >= capacity
	})
	if idx >= 0 {
		return &p.secrets[idx], nil
	}
	return p.newSecret(), nil
}

func (p *payloadSecrets) newSecret() *corev1.Secret {
	secret := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:   fmt.Sprintf("%s-payloads-%d", p.secretNamePrefix, len(p.secrets)),
			Labels: p.secretLabels,
		},
		StringData: map[string]string{},
	}
	p.secrets = append(p.secrets, secret)
	return &p.secrets[len(p.secrets)-1]
}

func (p *payloadSecrets) Secrets() []corev1.Secret {
	return p.secrets
}

func (p *payloadSecrets) getMaxPayloadDataSize() int {
	if p.maxDataSize > 0 {
		return p.maxDataSize
	}
	return defaultMaxPayloadDataSize
}
