package api

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/trust"
	"github.com/canonical/lxd/shared/ws"
	"github.com/canonical/microcluster/v2/rest"
	"github.com/canonical/microcluster/v2/state"
	"golang.org/x/sync/errgroup"

	"github.com/canonical/microcloud/microcloud/api/types"
	cloudClient "github.com/canonical/microcloud/microcloud/client"
	cloudMDNS "github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

// HMACMicroCloud10 is the HMAC format version used during trust establishment.
const HMACMicroCloud10 trust.HMACVersion = "MicroCloud-1.0"

// SessionInitiatingCmd represents the /1.0/session/initiating API on MicroCloud.
var SessionInitiatingCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "session/initiating",
		Path:              "session/initiating",

		Get: rest.EndpointAction{Handler: authHandlerMTLS(sh, sessionGet(sh, types.SessionInitiating))},
	}
}

// SessionJoiningCmd represents the /1.0/session/joining API on MicroCloud.
var SessionJoiningCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "session/joining",
		Path:              "session/joining",

		Get: rest.EndpointAction{Handler: authHandlerMTLS(sh, sessionGet(sh, types.SessionJoining))},
	}
}

// sessionGet returns a MicroCloud join session.
func sessionGet(sh *service.Handler, sessionRole types.SessionRole) func(state state.State, r *http.Request) response.Response {
	return func(state state.State, r *http.Request) response.Response {
		if sh.ActiveSession() {
			return response.BadRequest(errors.New("There already is an active session"))
		}

		sessionTimeoutStr := r.URL.Query().Get("timeout")
		if sessionTimeoutStr == "" {
			sessionTimeoutStr = "10m"
		}

		sessionTimeout, err := time.ParseDuration(sessionTimeoutStr)
		if err != nil {
			return response.BadRequest(fmt.Errorf("Failed to parse timeout: %w", err))
		}

		return response.ManualResponse(func(w http.ResponseWriter) error {
			conn, err := ws.Upgrader.Upgrade(w, r, nil)
			if err != nil {
				return err
			}

			defer func() {
				err := conn.Close()
				if err != nil {
					logger.Error("Failed to close the websocket connection", logger.Ctx{"err": err})
				}
			}()

			sessionCtx, cancel := context.WithTimeoutCause(r.Context(), sessionTimeout, errors.New("Session timeout exceeded"))
			defer cancel()

			gw := cloudClient.NewWebsocketGateway(sessionCtx, conn)

			if sessionRole == types.SessionInitiating {
				err = handleInitiatingSession(state, sh, gw)
			} else if sessionRole == types.SessionJoining {
				err = handleJoiningSession(state, sh, gw)
			}

			// Any errors occurring after the connection got upgraded have to be handled
			// within the websocket.
			// When writing a response to the original HTTP connection the server will
			// complain with "http: connection has been hijacked".
			if err != nil {
				controlErr := gw.WriteClose(err)
				if controlErr != nil {
					logger.Error("Failed to write close control message", logger.Ctx{"err": controlErr, "controlErr": err})
				}
			}

			return nil
		})
	}
}

func confirmedIntents(sh *service.Handler, gw *cloudClient.WebsocketGateway) ([]types.SessionJoinPost, error) {
	for {
		select {
		case intent := <-sh.Session.IntentCh():
			err := gw.Write(types.Session{
				Intent: intent,
			})
			if err != nil {
				return nil, fmt.Errorf("Failed to forward join intent: %w", err)
			}

		case bytes := <-gw.Receive():
			var session types.Session
			err := json.Unmarshal(bytes, &session)
			if err != nil {
				return nil, fmt.Errorf("Failed to read confirmed intents: %w", err)
			}

			return session.ConfirmedIntents, nil
		case <-gw.Context().Done():
			return nil, fmt.Errorf("Exit waiting for intents: %w", context.Cause(gw.Context()))
		}
	}
}

func handleInitiatingSession(state state.State, sh *service.Handler, gw *cloudClient.WebsocketGateway) error {
	session := types.Session{}
	err := gw.ReceiveWithContext(gw.Context(), &session)
	if err != nil {
		return fmt.Errorf("Failed to read session start message: %w", err)
	}

	err = sh.StartSession(session.Passphrase)
	if err != nil {
		return fmt.Errorf("Failed to start session: %w", err)
	}

	defer func() {
		err := sh.StopSession()
		if err != nil {
			logger.Error("Failed to stop session", logger.Ctx{"err": err})
		}
	}()

	sessionPassphrase := sh.Session.Passphrase()
	err = gw.Write(types.Session{
		Passphrase: sessionPassphrase,
	})
	if err != nil {
		return fmt.Errorf("Failed to send session details: %w", err)
	}

	err = sh.Session.Broadcast(state.Name(), session.Address, session.Interface)
	if err != nil {
		return fmt.Errorf("Failed to start broadcast: %w", err)
	}

	confirmedIntents, err := confirmedIntents(sh, gw)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(context.Background())

	// Add systems to temporary truststore.
	for _, intent := range confirmedIntents {
		remoteCert, err := shared.ParseCert([]byte(intent.Certificate))
		if err != nil {
			return fmt.Errorf("Failed to parse certificate of confirmed intent: %w", err)
		}

		// Add system to temporary truststore.
		sh.Session.Allow(intent.Name, *remoteCert)

		cloud := sh.Services[types.MicroCloud].(*service.CloudService)
		cert, err := cloud.ServerCert()
		if err != nil {
			return err
		}

		joinIntent := types.SessionJoinPost{
			Version:     cloudMDNS.Version,
			Name:        state.Name(),
			Address:     session.Address,
			Certificate: string(cert.PublicKey()),
			Services:    session.Services,
		}

		h, err := trust.NewHMACArgon2([]byte(sessionPassphrase), nil, trust.NewDefaultHMACConf(HMACMicroCloud10))
		if err != nil {
			return err
		}

		header, err := trust.HMACAuthorizationHeader(h, joinIntent)
		if err != nil {
			return err
		}

		// Confirm join intent.
		// This request uses polling to wait for confirmation from the other side.
		g.Go(func() error {
			conf := cloudClient.AuthConfig{
				HMAC: header,
				// We already know the certificate of the joiner for TLS verification.
				TLSServerCertificate: remoteCert,
			}

			_, err := cloud.RequestJoinIntent(ctx, intent.Address, conf, joinIntent)
			return err
		})
	}

	err = g.Wait()
	if err != nil {
		return fmt.Errorf("Failed to confirm join intents: %w", err)
	}

	err = gw.Write(types.Session{
		Accepted: true,
	})
	if err != nil {
		return fmt.Errorf("Failed to send confirmation: %w", err)
	}

	return nil
}

func handleJoiningSession(state state.State, sh *service.Handler, gw *cloudClient.WebsocketGateway) error {
	session := types.Session{}
	err := gw.ReceiveWithContext(gw.Context(), &session)
	if err != nil {
		return fmt.Errorf("Failed to read session start message: %w", err)
	}

	err = sh.StartSession(session.Passphrase)
	if err != nil {
		return fmt.Errorf("Failed to start session: %w", err)
	}

	defer func() {
		err := sh.StopSession()
		if err != nil {
			logger.Error("Failed to stop session", logger.Ctx{"err": err})
		}
	}()

	iface, err := net.InterfaceByName(session.Interface)
	if err != nil {
		return fmt.Errorf("Failed to lookup interface by name: %w", err)
	}

	// No address selected, try to lookup system.
	if session.InitiatorAddress == "" {
		lookupCtx, cancel := context.WithTimeout(gw.Context(), session.LookupTimeout)
		defer cancel()

		peer, err := cloudMDNS.LookupPeer(lookupCtx, iface, cloudMDNS.Version)
		if err != nil {
			return err
		}

		session.InitiatorAddress = peer.Address
	}

	// Get the remotes name.
	cloud := sh.Services[types.MicroCloud].(*service.CloudService)
	cert, err := cloud.ServerCert()
	if err != nil {
		return err
	}

	joinIntent := types.SessionJoinPost{
		Version:     cloudMDNS.Version,
		Name:        state.Name(),
		Address:     session.Address,
		Certificate: string(cert.PublicKey()),
		Services:    session.Services,
	}

	h, err := trust.NewHMACArgon2([]byte(session.Passphrase), nil, trust.NewDefaultHMACConf(HMACMicroCloud10))
	if err != nil {
		return err
	}

	header, err := trust.HMACAuthorizationHeader(h, joinIntent)
	if err != nil {
		return err
	}

	conf := cloudClient.AuthConfig{
		HMAC: header,
		// The certificate of the initiater isn't yet known so we have to skip any TLS verification.
		InsecureSkipVerify: true,
	}

	peerCert, err := cloud.RequestJoinIntent(context.Background(), session.InitiatorAddress, conf, joinIntent)
	if err != nil {
		fmt.Println("error request join intent", err)
		return err
	}

	session.InitiatorFingerprint = shared.CertFingerprint(peerCert)

	peerStatus, err := cloud.RemoteStatus(gw.Context(), peerCert, session.InitiatorAddress)
	if err != nil {
		return fmt.Errorf("Failed to retrieve cluster status: %w", err)
	}

	session.InitiatorName = peerStatus.Name

	// Notify the client we have found an eligible system.
	err = gw.Write(types.Session{
		InitiatorName:        session.InitiatorName,
		InitiatorAddress:     session.InitiatorAddress,
		InitiatorFingerprint: session.InitiatorFingerprint,
	})
	if err != nil {
		return fmt.Errorf("Failed to send the target address: %w", err)
	}

	var confirmedIntent types.SessionJoinPost

	select {
	case confirmedIntent = <-sh.Session.IntentCh():
		err = gw.Write(types.Session{
			Intent: confirmedIntent,
		})
		if err != nil {
			return fmt.Errorf("Failed to forward join confirmation: %w", err)
		}

	case <-gw.Context().Done():
		return fmt.Errorf("Exit waiting for join confirmation: %w", context.Cause(gw.Context()))
	}

	certBlock, _ := pem.Decode([]byte(confirmedIntent.Certificate))
	if certBlock == nil {
		return fmt.Errorf("Invalid certificate file")
	}

	remoteCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("Failed to parse certificate: %w", err)
	}

	// Add system to temporary truststore.
	sh.Session.Allow(confirmedIntent.Name, *remoteCert)

	var errStr string
	select {
	case <-sh.Session.ExitCh():
		errStr = ""
	case <-gw.Context().Done():
		errStr = fmt.Errorf("Exit waiting for session to end: %w", context.Cause(gw.Context())).Error()
	}

	err = gw.Write(types.Session{
		Error: errStr,
	})
	if err != nil {
		return fmt.Errorf("Failed to signal final message: %w", err)
	}

	return nil
}