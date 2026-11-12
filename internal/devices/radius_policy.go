package devices

import (
	"context"

	"github.com/google/uuid"
)

// RADIUSPolicyRecord is the active authorization policy for one device.
// PrivatePassword is only for the authenticated RADIUS snapshot formatter.
type RADIUSPolicyRecord struct {
	MACAddress      string
	VLANID          int32
	PrivatePassword string `json:"-"`
}

func (s *Service) RADIUSPolicy(ctx context.Context) ([]RADIUSPolicyRecord, error) {
	records, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	policy := make([]RADIUSPolicyRecord, 0, len(records))
	for _, record := range records {
		device := record.Device
		if device.Disabled {
			continue
		}
		entry := RADIUSPolicyRecord{MACAddress: device.MacAddress, VLANID: device.VlanID}
		if device.PrivateWifiPasswordEnabled {
			entry.PrivatePassword, err = s.passwordCipher.decrypt(uuid.UUID(device.ID.Bytes), device.WifiPasswordCiphertext)
			if err != nil {
				return nil, err
			}
			if err := validateWiFiPassword(entry.PrivatePassword); err != nil {
				return nil, err
			}
		}
		policy = append(policy, entry)
	}
	return policy, nil
}
