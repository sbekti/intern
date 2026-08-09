package httpserver

import (
	"errors"
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/clientauth"
	"github.com/sbekti/intern-api/internal/db"
	"github.com/sbekti/intern-api/internal/devices"
	"github.com/sbekti/intern-api/internal/vlans"
)

func handleVLANError(w http.ResponseWriter, err error) {
	var validationErr vlans.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeAPIError(w, http.StatusBadRequest, "bad_request", validationErr.Error())
	case errors.Is(err, vlans.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "vlan not found")
	case errors.Is(err, vlans.ErrReferencedByDevices):
		writeAPIError(w, http.StatusConflict, "conflict", "this VLAN is still assigned to one or more devices; reassign or delete those devices before deleting the VLAN")
	case errors.Is(err, vlans.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", "vlan conflicts with an existing record")
	default:
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan operation failed")
	}
}

func toAPIVlan(value db.Vlan) api.Vlan {
	return api.Vlan{
		Name:        value.Name,
		VlanId:      value.VlanID,
		Description: value.Description,
		CreatedAt:   value.CreatedAt.Time,
		UpdatedAt:   value.UpdatedAt.Time,
	}
}

func handleDeviceError(w http.ResponseWriter, err error) {
	var validationErr devices.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeAPIError(w, http.StatusBadRequest, "bad_request", validationErr.Error())
	case errors.Is(err, devices.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "device not found")
	case errors.Is(err, devices.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", "device conflicts with an existing record")
	default:
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "device operation failed")
	}
}

func toAPINetworkDevice(record devices.DeviceRecord) api.NetworkDevice {
	result := api.NetworkDevice{
		Id:          openapi_types.UUID(record.Device.ID.Bytes),
		MacAddress:  record.Device.MacAddress,
		DisplayName: record.Device.DisplayName,
		Disabled:    record.Device.Disabled,
		Vlan: api.VlanRef{
			Name:   record.VLAN.Name,
			VlanId: record.VLAN.VlanID,
		},
		CreatedAt: record.Device.CreatedAt.Time,
		UpdatedAt: record.Device.UpdatedAt.Time,
	}

	if record.Device.CreatedByUserID.Valid {
		value := openapi_types.UUID(record.Device.CreatedByUserID.Bytes)
		result.CreatedByUserId = &value
	}
	if record.Device.UpdatedByUserID.Valid {
		value := openapi_types.UUID(record.Device.UpdatedByUserID.Bytes)
		result.UpdatedByUserId = &value
	}

	return result
}

func handleClientAuthError(w http.ResponseWriter, err error) {
	var validationErr clientauth.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeAPIError(w, http.StatusBadRequest, "bad_request", validationErr.Error())
	case errors.Is(err, clientauth.ErrNotFound):
		writeAPIError(w, http.StatusNotFound, "not_found", "authorization request not found")
	case errors.Is(err, clientauth.ErrConflict):
		writeAPIError(w, http.StatusConflict, "conflict", "authorization request conflicts with current state")
	case errors.Is(err, clientauth.ErrAuthorizationPending):
		writeJSON(w, http.StatusPreconditionRequired, api.ClientAuthError{
			Error:            api.AuthorizationPending,
			ErrorDescription: "device code request is still pending approval",
		})
	case errors.Is(err, clientauth.ErrSlowDown):
		writeJSON(w, http.StatusBadRequest, api.ClientAuthError{
			Error:            api.SlowDown,
			ErrorDescription: "device code request is being polled too quickly",
		})
	case errors.Is(err, clientauth.ErrExpiredToken):
		writeJSON(w, http.StatusBadRequest, api.ClientAuthError{
			Error:            api.ExpiredToken,
			ErrorDescription: "device code request has expired",
		})
	case errors.Is(err, clientauth.ErrAccessDenied):
		writeJSON(w, http.StatusBadRequest, api.ClientAuthError{
			Error:            api.AccessDenied,
			ErrorDescription: "device code request was denied",
		})
	case errors.Is(err, clientauth.ErrInvalidRequest):
		writeJSON(w, http.StatusBadRequest, api.ClientAuthError{
			Error:            api.InvalidRequest,
			ErrorDescription: "device code request is invalid",
		})
	case errors.Is(err, clientauth.ErrUnauthorized):
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "refresh token is invalid")
	case errors.Is(err, clientauth.ErrTooManyRequests):
		writeAPIError(w, http.StatusTooManyRequests, "too_many_requests", "too many requests")
	default:
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "client auth operation failed")
	}
}
