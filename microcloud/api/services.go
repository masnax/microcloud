package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/canonical/microcluster/microcluster"
	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/state"
	"github.com/lxc/lxd/client"
	"github.com/lxc/lxd/lxd/response"

	"github.com/canonical/microcloud/microcloud/db"
)

// LXDProxy proxies all requests from MicroCloud to LXD.
var LXDProxy = Proxy("lxd", "services/lxd/{rest:.*}", lxdHandler)

// CephProxy proxies all requests from MicroCloud to Ceph.
var CephProxy = Proxy("ceph", "services/ceph/{rest:.*}", cephHandler)

// Proxy returns a proxy endpoint with the given handler and access applied to all REST methods.
func Proxy(name, path string, handler func(*state.State, *http.Request) response.Response) rest.Endpoint {
	return rest.Endpoint{
		Name: name,
		Path: path,

		Get:    rest.EndpointAction{Handler: handler, AllowUntrusted: true, ProxyTarget: true},
		Put:    rest.EndpointAction{Handler: handler, AllowUntrusted: true, ProxyTarget: true},
		Post:   rest.EndpointAction{Handler: handler, AllowUntrusted: true, ProxyTarget: true},
		Patch:  rest.EndpointAction{Handler: handler, AllowUntrusted: true, ProxyTarget: true},
		Delete: rest.EndpointAction{Handler: handler, AllowUntrusted: true, ProxyTarget: true},
	}
}

// lxdHandler forwards a request made to /1.0/services/lxd/<rest> to /1.0/<rest> on the LXD unix socket.
func lxdHandler(s *state.State, r *http.Request) response.Response {
	_, path, ok := strings.Cut(r.URL.Path, "/1.0/services/lxd/")
	if !ok {
		return response.SmartError(fmt.Errorf("Invalid path %q", r.URL.Path))
	}

	if path == "" {
		r.URL.Path = "/1.0"
	} else {
		r.URL.Path = "/1.0/" + path
	}

	var dir string
	err := s.Database.Transaction(s.Context, func(ctx context.Context, tx *sql.Tx) error {
		service, err := db.GetService(ctx, tx, db.LXD)
		if err != nil {
			return err
		}

		dir = service.StateDir

		return nil
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get LXD state directory: %w", err))
	}

	// Must unset the RequestURI. It is an error to set this in a client request.
	r.RequestURI = ""
	r.URL.Scheme = "http"
	r.URL.Host = filepath.Join(dir, "unix.socket")
	r.Host = r.URL.Host
	client, err := lxd.ConnectLXDUnix(filepath.Join(dir, "unix.socket"), nil)
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to connect to local LXD: %w", err))
	}

	resp, err := client.DoHTTP(r)
	if err != nil {
		return response.SmartError(err)
	}

	return NewResponse(resp)
}

// cephHandler forwards a request made to /1.0/services/ceph/<rest> to /1.0/<rest> on the MicroCeph unix socket.
func cephHandler(s *state.State, r *http.Request) response.Response {
	_, path, ok := strings.Cut(r.URL.Path, "/1.0/services/ceph/")
	if !ok {
		return response.SmartError(fmt.Errorf("Invalid path %q", r.URL.Path))
	}

	if path == "" {
		r.URL.Path = "/1.0"
	} else {
		r.URL.Path = "/1.0/" + path
	}

	var dir string
	err := s.Database.Transaction(s.Context, func(ctx context.Context, tx *sql.Tx) error {
		service, err := db.GetService(ctx, tx, db.MicroCeph)
		if err != nil {
			return err
		}

		dir = service.StateDir

		return nil
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get MicroCeph state directory: %w", err))
	}

	// Must unset the RequestURI. It is an error to set this in a client request.
	r.RequestURI = ""
	r.URL.Scheme = "http"
	r.URL.Host = filepath.Join(dir, "control.socket")
	r.Host = r.URL.Host
	client, err := microcluster.App(s.Context, dir, false, false)
	if err != nil {
		return response.SmartError(err)
	}

	c, err := client.LocalClient()
	if err != nil {
		return response.SmartError(err)
	}

	resp, err := c.Do(r)
	if err != nil {
		return response.SmartError(err)
	}

	return NewResponse(resp)
}
