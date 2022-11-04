package service

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/canonical/microcluster/client"
	"github.com/canonical/microcluster/microcluster"
	"github.com/lxc/lxd/lxd/util"
	"github.com/lxc/lxd/shared"

	cloudClient "github.com/canonical/microcloud/microcloud/client"
)

// CephService is a MicroCeph service.
type CephService struct {
	m *microcluster.MicroCluster

	name    string
	address string
	port    int
}

// NewCephService creates a new MicroCeph service with a client attached.
func NewCephService(ctx context.Context, name string, addr string, cloudDir string) (*CephService, error) {
	proxy := func(r *http.Request) (*url.URL, error) {
		if !strings.HasPrefix(r.URL.Path, "/1.0/services/microceph") {
			r.URL.Path = "/1.0/services/microceph" + r.URL.Path
		}

		return shared.ProxyFromEnvironment(r)
	}

	client, err := microcluster.App(ctx, microcluster.Args{StateDir: cloudDir, Proxy: proxy})
	if err != nil {
		return nil, err
	}

	return &CephService{
		m:       client,
		name:    name,
		address: addr,
		port:    CephPort,
	}, nil
}

// client returns a client to the Ceph unix socket.
func (s CephService) Client() (*client.Client, error) {
	c, err := s.m.LocalClient()
	if err != nil {
		return nil, err
	}

	return cloudClient.NewCephClient(c), nil
}

// Bootstrap bootstraps the MicroCeph daemon on the default port.
func (s CephService) Bootstrap() error {
	return s.m.NewCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), time.Second*30)
}

// IssueToken issues a token for the given peer.
func (s CephService) IssueToken(peer string) (string, error) {
	return s.m.NewJoinToken(peer)
}

// Join joins a cluster with the given token.
func (s CephService) Join(token string) error {
	return s.m.JoinCluster(s.name, util.CanonicalNetworkAddress(s.address, s.port), token, time.Second*30)
}

// Type returns the type of Service.
func (s CephService) Type() ServiceType {
	return MicroCeph
}

// Name returns the name of this Service instance.
func (s CephService) Name() string {
	return s.name
}

// Address returns the address of this Service instance.
func (s CephService) Address() string {
	return s.address
}

// Port returns the port of this Service instance.
func (s CephService) Port() int {
	return s.port
}
