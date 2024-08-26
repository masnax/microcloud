package api

import (
	"fmt"
	"net/http"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/trust"
	"github.com/canonical/microcluster/v2/state"

	"github.com/canonical/microcloud/microcloud/service"
)

// endpointHandler is just a convenience for writing clean return types.
type endpointHandler func(state.State, *http.Request) response.Response

// authHandlerMTLS ensures a request has been authenticated using mTLS.
func authHandlerMTLS(sh *service.Handler, f endpointHandler) endpointHandler {
	return func(s state.State, r *http.Request) response.Response {
		if r.RemoteAddr == "@" {
			logger.Debug("Allowing unauthenticated request through unix socket")

			return f(s, r)
		}

		// Use certificate based authentication between cluster members.
		if r.TLS != nil {
			trustedCerts := s.Remotes().CertificatesNative()
			for _, cert := range r.TLS.PeerCertificates {
				// First evaluate the permanent turst store.
				trusted, _ := util.CheckMutualTLS(*cert, trustedCerts)
				if trusted {
					return f(s, r)
				}

				// Second evaluate the temporary trust store.
				// This is the fallback during the forming of the cluster.
				trusted, _ = util.CheckMutualTLS(*cert, sh.TemporaryTrustStore())
				if trusted {
					return f(s, r)
				}
			}
		}

		return response.Forbidden(fmt.Errorf("Failed to authenticate using mTLS"))
	}
}

// authHandlerHMAC ensures a request has been authenticated using the HMAC in the Authorization header.
func authHandlerHMAC(sh *service.Handler, f endpointHandler) endpointHandler {
	return func(s state.State, r *http.Request) response.Response {
		if !sh.ActiveSession() {
			return response.BadRequest(fmt.Errorf("No active session"))
		}

		h, err := trust.NewHMACArgon2([]byte(sh.Session.Passphrase()), nil, trust.NewDefaultHMACConf(HMACMicroCloud10))
		if err != nil {
			return response.SmartError(err)
		}

		err = trust.HMACEqual(h, r)
		if err != nil {
			return response.SmartError(err)
		}

		return f(s, r)
	}
}
