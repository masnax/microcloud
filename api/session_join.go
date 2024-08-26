package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/service"
)

// SessionJoinCmd represents the /1.0/session/join API on MicroCloud.
var SessionJoinCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "session/join",
		Path:              "session/join",

		Post: rest.EndpointAction{Handler: authHandlerHMAC(sh, sessionJoinPost(sh)), AllowUntrusted: true},
	}
}

// sessionJoinPost receives join intent requests from new potential members.
func sessionJoinPost(sh *service.Handler) func(state state.State, r *http.Request) response.Response {
	return func(state state.State, r *http.Request) response.Response {
		// Parse the request.
		req := types.SessionJoinPost{}

		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			return response.BadRequest(err)
		}

		if !sh.ActiveSession() {
			return response.InternalError(fmt.Errorf("No active join session"))
		}

		err = validateIntent(sh, req)
		if err != nil {
			return response.BadRequest(err)
		}

		finerprint, err := shared.CertFingerprintStr(req.Certificate)
		if err != nil {
			return response.BadRequest(fmt.Errorf("Failed to get fingerprint: %w", err))
		}

		err = sh.Session.RegisterIntent(finerprint)
		if err != nil {
			return response.BadRequest(fmt.Errorf("Failed to register join intent: %w", err))
		}

		// Prevent locking in case there isn't anymore an active consumer reading on the channel.
		// This can happen if the initiator's websocket connection isn't anymore active.
		select {
		case sh.Session.IntentCh() <- req:
			return response.EmptySyncResponse
		default:
			return response.InternalError(fmt.Errorf("No active consumer for join intent"))
		}
	}
}

// validateIntent validates the given join intent.
// It checks whether or not the peer is missing any of our services and returns an error if one is missing.
func validateIntent(sh *service.Handler, intent types.SessionJoinPost) error {
	// Reject any peers that are missing our services.
	for service := range sh.Services {
		if !shared.ValueInSlice(service, intent.Services) {
			return fmt.Errorf("Rejecting peer %q due to missing services (%s)", intent.Name, string(service))
		}
	}

	return nil
}
