package devices

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/sbekti/intern/internal/db"
)

func TestPasswordCipherBindsDeviceAndDetectsTampering(t *testing.T) {
	cipher, err := NewPasswordCipher(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	password := ` four words-$ENV{PATH}-'"\\ `
	sealed, err := cipher.encrypt(id, password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cipher.encrypt(id, password)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(sealed, second) || bytes.Contains(sealed, []byte(password)) {
		t.Fatal("ciphertext is deterministic or contains plaintext")
	}
	got, err := cipher.decrypt(id, sealed)
	if err != nil || got != password {
		t.Fatal("password did not round-trip literally")
	}
	restored, err := NewPasswordCipher(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := restored.decrypt(id, sealed); err != nil || got != password {
		t.Fatal("restoring the key did not recover the password")
	}
	if _, err := cipher.decrypt(uuid.New(), sealed); err == nil {
		t.Fatal("accepted another device's ciphertext")
	}
	wrongKey, _ := NewPasswordCipher(bytes.Repeat([]byte{9}, 32))
	if _, err := wrongKey.decrypt(id, sealed); err == nil {
		t.Fatal("accepted another encryption key")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := cipher.decrypt(id, sealed); err == nil {
		t.Fatal("accepted tampered ciphertext")
	}
	if _, err := cipher.decrypt(id, []byte{2}); err == nil {
		t.Fatal("accepted an unknown format version")
	}
}

func TestWiFiPasswordValidationAndGeneration(t *testing.T) {
	for _, value := range []string{"1234567", string(bytes.Repeat([]byte{'a'}, 64)), "has\na-newline", "caf\u00e9-long-password"} {
		if validateWiFiPassword(value) == nil {
			t.Fatal("accepted an invalid password")
		}
	}
	for _, value := range []string{"12345678", " spaces stay ", string(bytes.Repeat([]byte{'a'}, 63))} {
		if err := validateWiFiPassword(value); err != nil {
			t.Fatal(err)
		}
	}
	seen := make(map[string]bool)
	for range 10 {
		password, err := GenerateWiFiPassword()
		if err != nil {
			t.Fatal(err)
		}
		if err := validateWiFiPassword(password); err != nil {
			t.Fatal(err)
		}
		if seen[password] {
			t.Fatal("generator repeated a passphrase")
		}
		seen[password] = true
	}
}

func TestRevealReturnsNothingWhenAuditFails(t *testing.T) {
	cipher, _ := NewPasswordCipher(bytes.Repeat([]byte{7}, 32))
	id := uuid.New()
	sealed, _ := cipher.encrypt(id, "forest-river-cloud-moon")
	queries := fakeQuerier{
		getDeviceFn: func(context.Context, db.GetNetworkDeviceByIDParams) (db.NetworkDevice, error) {
			return db.NetworkDevice{WifiPasswordCiphertext: sealed}, nil
		},
		createAuditLogFn: func(context.Context, db.CreateAuditLogParams) (db.AuditLog, error) {
			return db.AuditLog{}, errors.New("audit unavailable")
		},
	}
	service := NewService(queries, fakeTransactor{q: queries}, cipher)
	password, err := service.RevealWiFiPassword(context.Background(), db.User{}, id)
	if err == nil || password != "" {
		t.Fatal("revealed a password without an audit record")
	}
}
