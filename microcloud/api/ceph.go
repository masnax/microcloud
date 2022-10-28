package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/canonical/microcluster/microcluster"
	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/state"
	"github.com/lxc/lxd/lxd/response"

	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/db"
)

// CephControlCmd handles any request to the MicroCeph /cluster/control endpoint.
var CephControlCmd = rest.Endpoint{
	Path: "services/ceph/control",

	Post: rest.EndpointAction{Handler: cephControlPost, AllowUntrusted: true},
}

// CephTokensCmd handles any request to the MicroCeph /cluster/1.0/tokens endpoint.
var CephTokensCmd = rest.Endpoint{
	Path: "services/ceph/tokens",

	Post: rest.EndpointAction{Handler: cephTokensPost, AllowUntrusted: true},
}

// CephClusterCmd handles any request to the MicroCeph /cluster/1.0/cluster endpoint.
var CephClusterCmd = rest.Endpoint{
	Path: "services/ceph/cluster",

	Post:   CephProxy.Post,
	Put:    CephProxy.Put,
	Patch:  CephProxy.Patch,
	Get:    rest.EndpointAction{Handler: cephClusterGet, AllowUntrusted: true},
	Delete: CephProxy.Delete,
}

func getCephClient(state *state.State) (*microcluster.MicroCluster, error) {
	var dir string
	err := state.Database.Transaction(state.Context, func(ctx context.Context, tx *sql.Tx) error {
		service, err := db.GetService(ctx, tx, db.MicroCeph)
		if err != nil {
			return err
		}

		dir = service.StateDir

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to get MicroCeph state directory: %w", err)
	}

	return microcluster.App(state.Context, dir, false, false)
}

func cephControlPost(state *state.State, r *http.Request) response.Response {
	m, err := getCephClient(state)
	if err != nil {
		return response.SmartError(err)
	}

	var data client.ControlPost
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return response.SmartError(err)
	}

	if data.Bootstrap {
		err := m.NewCluster(data.Name, data.Address.String(), time.Second*30)
		if err != nil {
			return response.SmartError(err)
		}

		return response.EmptySyncResponse
	} else {
		err := m.JoinCluster(data.Name, data.Address.String(), data.JoinToken, time.Second*30)
		if err != nil {
			return response.SmartError(err)
		}

		return response.EmptySyncResponse
	}
}

func cephTokensPost(state *state.State, r *http.Request) response.Response {
	m, err := getCephClient(state)
	if err != nil {
		return response.SmartError(err)
	}

	var data client.TokenRecord
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		return response.SmartError(err)
	}

	token, err := m.NewJoinToken(data.Name)
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, token)
}

func cephClusterGet(state *state.State, r *http.Request) response.Response {
	m, err := getCephClient(state)
	if err != nil {
		return response.SmartError(err)
	}

	c, err := m.LocalClient()
	if err != nil {
		return response.SmartError(err)
	}

	internalMembers, err := c.GetClusterMembers(state.Context)
	if err != nil {
		return response.SmartError(err)
	}

	members := make([]client.ClusterMember, 0, len(internalMembers))
	for _, member := range internalMembers {
		members = append(members, client.ClusterMember{
			ClusterMemberLocal: client.ClusterMemberLocal{
				Name:        member.Name,
				Address:     member.Address,
				Certificate: member.Certificate,
			},
			Role:          member.Role,
			SchemaVersion: member.SchemaVersion,
			LastHeartbeat: member.LastHeartbeat,
			Status:        string(member.Status),
			Secret:        member.Secret,
		})
	}

	return response.SyncResponse(true, members)
}
