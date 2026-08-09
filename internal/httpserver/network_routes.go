package httpserver

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/auth"
	"github.com/sbekti/intern/internal/identity"
	"github.com/sbekti/intern/internal/vlans"
)

func registerNetworkRoutes(
	r chi.Router,
	authorizer *auth.Authorizer,
	vlanService VLANService,
	deviceService DeviceService,
) {
	r.With(authorizer.RequireAdmin()).Get("/networks/vlans", func(w http.ResponseWriter, r *http.Request) {
		if vlanService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan service not configured")
			return
		}

		items, err := vlanService.List(r.Context())
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list vlans")
			return
		}

		responseItems := make([]api.Vlan, 0, len(items))
		for _, item := range items {
			responseItems = append(responseItems, toAPIVlan(item))
		}

		writeJSON(w, http.StatusOK, api.VlanList{Items: responseItems})
	})

	r.With(authorizer.RequireAdmin()).Get("/networks/vlans/{vlan_id}", func(w http.ResponseWriter, r *http.Request) {
		if vlanService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan service not configured")
			return
		}

		vlanID, err := decodeInt32PathParam(r, "vlan_id")
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid vlan id")
			return
		}

		vlan, err := vlanService.Get(r.Context(), vlanID)
		if err != nil {
			if errors.Is(err, vlans.ErrNotFound) {
				writeAPIError(w, http.StatusNotFound, "not_found", "vlan not found")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load vlan")
			return
		}

		writeJSON(w, http.StatusOK, toAPIVlan(vlan))
	})

	r.With(authorizer.RequireAdmin()).Post("/networks/vlans", func(w http.ResponseWriter, r *http.Request) {
		if vlanService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan service not configured")
			return
		}

		actor, ok := identity.FromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
			return
		}

		var body api.VlanWrite
		if err := decodeJSON(r, &body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		created, err := vlanService.Create(r.Context(), actor, body)
		if err != nil {
			handleVLANError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, toAPIVlan(created))
	})

	r.With(authorizer.RequireAdmin()).Patch("/networks/vlans/{vlan_id}", func(w http.ResponseWriter, r *http.Request) {
		if vlanService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan service not configured")
			return
		}

		actor, ok := identity.FromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
			return
		}

		vlanID, err := decodeInt32PathParam(r, "vlan_id")
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid vlan id")
			return
		}

		var body api.VlanPatch
		if err := decodeJSON(r, &body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		updated, err := vlanService.Update(r.Context(), actor, vlanID, body)
		if err != nil {
			handleVLANError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, toAPIVlan(updated))
	})

	r.With(authorizer.RequireAdmin()).Delete("/networks/vlans/{vlan_id}", func(w http.ResponseWriter, r *http.Request) {
		if vlanService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "vlan service not configured")
			return
		}

		actor, ok := identity.FromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
			return
		}

		vlanID, err := decodeInt32PathParam(r, "vlan_id")
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid vlan id")
			return
		}

		if err := vlanService.Delete(r.Context(), actor, vlanID); err != nil {
			handleVLANError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	r.With(authorizer.RequireAdmin()).Get("/networks/devices", func(w http.ResponseWriter, r *http.Request) {
		if deviceService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "device service not configured")
			return
		}

		items, err := deviceService.List(r.Context())
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list devices")
			return
		}

		responseItems := make([]api.NetworkDevice, 0, len(items))
		for _, item := range items {
			responseItems = append(responseItems, toAPINetworkDevice(item))
		}

		writeJSON(w, http.StatusOK, api.NetworkDeviceList{Items: responseItems})
	})

	r.With(authorizer.RequireAdmin()).Get("/networks/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		if deviceService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "device service not configured")
			return
		}

		id, err := decodeUUIDPathParam(r, "id")
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid device id")
			return
		}

		record, err := deviceService.Get(r.Context(), id)
		if err != nil {
			handleDeviceError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, toAPINetworkDevice(record))
	})

	r.With(authorizer.RequireAdmin()).Post("/networks/devices", func(w http.ResponseWriter, r *http.Request) {
		if deviceService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "device service not configured")
			return
		}

		actor, ok := identity.FromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
			return
		}

		var body api.NetworkDeviceWrite
		if err := decodeJSON(r, &body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		record, err := deviceService.Create(r.Context(), actor, body)
		if err != nil {
			handleDeviceError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, toAPINetworkDevice(record))
	})

	r.With(authorizer.RequireAdmin()).Patch("/networks/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		if deviceService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "device service not configured")
			return
		}

		actor, ok := identity.FromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
			return
		}

		id, err := decodeUUIDPathParam(r, "id")
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid device id")
			return
		}

		var body api.NetworkDevicePatch
		if err := decodeJSON(r, &body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid request body")
			return
		}

		record, err := deviceService.Update(r.Context(), actor, id, body)
		if err != nil {
			handleDeviceError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, toAPINetworkDevice(record))
	})

	r.With(authorizer.RequireAdmin()).Delete("/networks/devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		if deviceService == nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "device service not configured")
			return
		}

		actor, ok := identity.FromContext(r.Context())
		if !ok {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "current user missing")
			return
		}

		id, err := decodeUUIDPathParam(r, "id")
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "bad_request", "invalid device id")
			return
		}

		if err := deviceService.Delete(r.Context(), actor, id); err != nil {
			handleDeviceError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
