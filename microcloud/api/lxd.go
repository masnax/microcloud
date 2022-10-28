package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/canonical/microcloud/microcloud/db"
	"github.com/canonical/microcluster/rest"
	"github.com/canonical/microcluster/state"
	"github.com/lxc/lxd/client"
	"github.com/lxc/lxd/lxd/response"
	"github.com/lxc/lxd/shared/api"
)

// LXDClusterCmd contains any alternatively handled methods on `/1.0/cluster` for LXD.
var LXDClusterCmd = rest.Endpoint{
	Path: "services/lxd/cluster",

	Put:    rest.EndpointAction{Handler: clusterPut, AllowUntrusted: true},
	Post:   LXDProxy.Post,
	Patch:  LXDProxy.Patch,
	Get:    LXDProxy.Get,
	Delete: LXDProxy.Delete,
}

// LXDClusterMemberCmd contains any alternatively handled methods on `/1.0/cluster/members` for LXD.
var LXDClusterMemberCmd = rest.Endpoint{
	Path: "services/lxd/cluster/members",

	Post:   rest.EndpointAction{Handler: clusterMemberPost, AllowUntrusted: true},
	Put:    LXDProxy.Put,
	Patch:  LXDProxy.Patch,
	Get:    LXDProxy.Get,
	Delete: LXDProxy.Delete,
}

// LXDProfilesCmd contains any alternatively handled methods on `/1.0/profiles` for LXD.
var LXDProfilesCmd = rest.Endpoint{
	Path: "services/lxd/profiles",

	Get:    rest.EndpointAction{Handler: profilesGet, AllowUntrusted: true},
	Post:   LXDProxy.Post,
	Put:    LXDProxy.Put,
	Patch:  LXDProxy.Patch,
	Delete: LXDProxy.Delete,
}

func getLXDClient(s *state.State) (lxd.InstanceServer, error) {
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
		return nil, fmt.Errorf("Failed to get LXD state directory: %w", err)
	}

	return lxd.ConnectLXDUnix(filepath.Join(dir, "unix.socket"), nil)
}

func clusterPut(state *state.State, r *http.Request) response.Response {
	client, err := getLXDClient(state)
	if err != nil {
		return response.SmartError(err)
	}

	var cluster api.ClusterPut
	err = json.NewDecoder(r.Body).Decode(&cluster)
	if err != nil {
		return response.SmartError(err)
	}

	op, err := client.UpdateCluster(cluster, "")
	if err != nil {
		return response.SmartError(err)
	}

	err = op.Wait()
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to configure cluster :%w", err))
	}

	return response.EmptySyncResponse
}

func clusterMemberPost(state *state.State, r *http.Request) response.Response {
	client, err := getLXDClient(state)
	if err != nil {
		return response.SmartError(err)
	}

	var cluster api.ClusterMembersPost
	err = json.NewDecoder(r.Body).Decode(&cluster)
	if err != nil {
		return response.SmartError(err)
	}

	op, err := client.CreateClusterMember(cluster)
	if err != nil {
		return response.SmartError(err)
	}

	opAPI := op.Get()
	joinToken, err := opAPI.ToClusterJoinToken()
	if err != nil {
		response.SmartError(fmt.Errorf("Failed converting token operation to join token: %w", err))
	}

	return response.SyncResponse(true, joinToken)
}

func profilesGet(state *state.State, r *http.Request) response.Response {
	client, err := getLXDClient(state)
	if err != nil {
		return response.SmartError(err)
	}

	names, err := client.GetProfileNames()
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, names)
}
