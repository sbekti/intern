package devices

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sbekti/intern/internal/db"
	"github.com/sethvargo/go-diceware/diceware"
)

var ErrPasswordKeyMissing = errors.New("Wi-Fi password encryption is not configured")

const passwordCipherVersion byte = 1

// PasswordCipher binds each stored password to its immutable device ID.
type PasswordCipher struct {
	aead cipher.AEAD
}

func NewPasswordCipher(key []byte) (*PasswordCipher, error) {
	if len(key) != 32 {
		return nil, errors.New("Wi-Fi password encryption key must contain 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, err
	}
	return &PasswordCipher{aead: aead}, nil
}

func (c *PasswordCipher) encrypt(id uuid.UUID, password string) ([]byte, error) {
	if c == nil {
		return nil, ErrPasswordKeyMissing
	}
	return c.aead.Seal([]byte{passwordCipherVersion}, nil, []byte(password), id[:]), nil
}

func (c *PasswordCipher) decrypt(id uuid.UUID, ciphertext []byte) (string, error) {
	if c == nil {
		return "", ErrPasswordKeyMissing
	}
	if len(ciphertext) == 0 || ciphertext[0] != passwordCipherVersion {
		return "", errors.New("unsupported Wi-Fi password ciphertext version")
	}
	password, err := c.aead.Open(nil, nil, ciphertext[1:], id[:])
	if err != nil {
		return "", errors.New("cannot decrypt Wi-Fi password")
	}
	return string(password), nil
}

func GenerateWiFiPassword() (string, error) {
	words, err := diceware.Generate(4)
	if err != nil {
		return "", fmt.Errorf("generate Wi-Fi password: %w", err)
	}
	return strings.Join(words, "-"), nil
}

func validateWiFiPassword(password string) error {
	if len(password) < 8 || len(password) > 63 {
		return ValidationError{Message: "Wi-Fi password must contain 8 to 63 printable ASCII characters"}
	}
	for _, char := range []byte(password) {
		if char < 0x20 || char > 0x7e {
			return ValidationError{Message: "Wi-Fi password must contain 8 to 63 printable ASCII characters"}
		}
	}
	return nil
}

func (s *Service) updateWiFiPassword(ctx context.Context, q Querier, device db.NetworkDevice, enabled *bool, password *string) (db.NetworkDevice, error) {
	if (enabled == nil || *enabled == device.PrivateWifiPasswordEnabled) && (password == nil || *password == "") {
		return device, nil
	}
	if enabled != nil {
		device.PrivateWifiPasswordEnabled = *enabled
	}
	if password != nil && *password != "" {
		if err := validateWiFiPassword(*password); err != nil {
			return db.NetworkDevice{}, err
		}
		var err error
		device.WifiPasswordCiphertext, err = s.passwordCipher.encrypt(uuid.UUID(device.ID.Bytes), *password)
		if err != nil {
			return db.NetworkDevice{}, err
		}
	}
	if device.PrivateWifiPasswordEnabled && len(device.WifiPasswordCiphertext) == 0 {
		return db.NetworkDevice{}, ValidationError{Message: "set a Wi-Fi password before enabling private access"}
	}
	if device.PrivateWifiPasswordEnabled && s.passwordCipher == nil {
		return db.NetworkDevice{}, ErrPasswordKeyMissing
	}
	return q.UpdateNetworkDeviceWiFi(ctx, db.UpdateNetworkDeviceWiFiParams{
		ID: device.ID, PrivateWifiPasswordEnabled: device.PrivateWifiPasswordEnabled,
		WifiPasswordCiphertext: device.WifiPasswordCiphertext,
	})
}

func wiFiAuditMetadata(device db.NetworkDevice, password *string) map[string]bool {
	return map[string]bool{
		"private_password_enabled": device.PrivateWifiPasswordEnabled,
		"password_set":             len(device.WifiPasswordCiphertext) > 0,
		"password_changed":         password != nil && *password != "",
	}
}

func (s *Service) RevealWiFiPassword(ctx context.Context, actor db.User, id uuid.UUID) (string, error) {
	if s == nil || s.tx == nil {
		return "", ErrTransactorMissing
	}
	var password string
	err := s.tx.InTx(ctx, func(q Querier) error {
		device, err := q.GetNetworkDeviceByID(ctx, db.GetNetworkDeviceByIDParams{ID: toPgUUID(id)})
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && len(device.WifiPasswordCiphertext) == 0) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		password, err = s.passwordCipher.decrypt(id, device.WifiPasswordCiphertext)
		if err != nil {
			return err
		}
		_, err = q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorUserID: actor.ID, ActorUsername: actor.Username,
			Action: "device.wifi_password.reveal", ResourceType: "network_device",
			ResourceID: id.String(), Metadata: []byte(`{}`),
		})
		return err
	})
	if err != nil {
		return "", err
	}
	return password, nil
}
