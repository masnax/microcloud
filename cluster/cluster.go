package cluster

import (
	"context"
	"fmt"
	"net"

	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/mdns"
	"github.com/canonical/microcloud/microcloud/service"
)

type SystemInformation struct {
	ExistingServices map[types.ServiceType]map[string]string
	ClusterName      string
	ClusterAddress   string
	AuthSecret       string

	AvailableDisks            map[string]api.ResourcesStorageDisk
	AvailableUplinkInterfaces map[string]api.Network
	AvailableCephInterfaces   map[string]service.CephDedicatedInterface

	LXDLocalConfig map[string]any
	LXDConfig      map[string]any
	CephConfig     map[string]string

	existingLocalPool    *api.StoragePool
	existingRemotePool   *api.StoragePool
	existingRemoteFSPool *api.StoragePool

	existingFanNetwork    *api.Network
	existingOVNNetwork    *api.Network
	existingUplinkNetwork *api.Network
}

// CollectSystemInformation fetches the current configuration of the server specified by the connection info.
func CollectSystemInformation(ctx context.Context, sh *service.Handler, connectInfo mdns.ServerInfo) (*SystemInformation, error) {
	if connectInfo.Name == "" || connectInfo.Address == "" {
		return nil, fmt.Errorf("Connection information is incomplete")
	}

	localSystem := sh.Name == connectInfo.Name

	s := &SystemInformation{
		ExistingServices:          map[types.ServiceType]map[string]string{},
		ClusterName:               connectInfo.Name,
		ClusterAddress:            connectInfo.Address,
		AuthSecret:                connectInfo.AuthSecret,
		AvailableDisks:            map[string]api.ResourcesStorageDisk{},
		AvailableUplinkInterfaces: map[string]api.Network{},
		AvailableCephInterfaces:   map[string]service.CephDedicatedInterface{},
	}

	var err error
	s.ExistingServices, err = GetExistingClusters(ctx, sh, connectInfo)
	if err != nil {
		return nil, err
	}

	var allResources *api.Resources
	lxd := sh.Services[types.LXD].(*service.LXDService)
	if localSystem {
		allResources, err = lxd.GetResources(ctx, s.ClusterName, "", "")
	} else {
		allResources, err = lxd.GetResources(ctx, s.ClusterName, s.ClusterAddress, s.AuthSecret)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to get system resources of peer %q: %w", s.ClusterName, err)
	}

	if allResources != nil {
		for _, disk := range allResources.Storage.Disks {
			if len(disk.Partitions) == 0 {
				s.AvailableDisks[disk.ID] = disk
			}
		}
	}

	var allNets []api.Network
	s.AvailableUplinkInterfaces, s.AvailableCephInterfaces, allNets, err = lxd.GetNetworkInterfaces(ctx, s.ClusterName, s.ClusterAddress, s.AuthSecret)
	if err != nil {
		return nil, err
	}

	for _, network := range allNets {
		if network.Name == "lxdfan0" {
			s.existingFanNetwork = &network
			continue
		}

		if network.Name == "default" {
			s.existingOVNNetwork = &network
			continue
		}

		if network.Name == "UPLINK" {
			s.existingUplinkNetwork = &network
			continue
		}
	}

	pools, err := lxd.GetStoragePools(ctx, s.ClusterName, s.ClusterAddress, s.AuthSecret)
	if err != nil {
		return nil, err
	}

	pool, ok := pools["local"]
	if ok {
		poolCopy := pool
		s.existingLocalPool = &poolCopy
	}

	pool, ok = pools["remote"]
	if ok {
		poolCopy := pool
		s.existingRemotePool = &poolCopy
	}

	pool, ok = pools["remote-fs"]
	if ok {
		poolCopy := pool
		s.existingRemoteFSPool = &poolCopy
	}

	// Last one is configs

	if len(s.ExistingServices[types.MicroCeph]) > 0 {
		microceph := sh.Services[types.MicroCeph].(*service.CephService)

		if localSystem {
			s.CephConfig, err = microceph.ClusterConfig(ctx, "", "")
		} else {
			s.CephConfig, err = microceph.ClusterConfig(ctx, s.ClusterAddress, s.AuthSecret)
		}
		if err != nil && err.Error() != "Daemon not yet initialized" {
			return nil, err
		}
	}

	if localSystem {
		s.LXDLocalConfig, s.LXDConfig, err = lxd.GetConfig(ctx, s.ServiceClustered(types.LXD), s.ClusterName, "", "")
	} else {
		s.LXDLocalConfig, s.LXDConfig, err = lxd.GetConfig(ctx, s.ServiceClustered(types.LXD), s.ClusterName, s.ClusterAddress, s.AuthSecret)
	}

	if err != nil {
		return nil, err
	}

	return s, nil
}

// GetExistingClusters checks against the services reachable by the specified ServerInfo,
// and returns a map of cluster members for each service supported by the Handler.
// If a service is not clustered, its map will be nil.
func GetExistingClusters(ctx context.Context, sh *service.Handler, connectInfo mdns.ServerInfo) (map[types.ServiceType]map[string]string, error) {
	localSystem := sh.Name == connectInfo.Name
	var err error
	existingServices := map[types.ServiceType]map[string]string{}
	for service := range sh.Services {
		var existingCluster map[string]string
		if localSystem {
			existingCluster, err = sh.Services[service].ClusterMembers(ctx)
		} else {
			existingCluster, err = sh.Services[service].RemoteClusterMembers(ctx, connectInfo.AuthSecret, connectInfo.Address)
		}

		if err != nil && err.Error() != "Daemon not yet initialized" && err.Error() != "Server is not clustered" {
			return nil, fmt.Errorf("Failed to reach %s on system %q: %w", service, connectInfo.Name, err)
		}

		// If a service isn't clustered, this loop will be skipped.

		for k, v := range existingCluster {
			if existingServices[service] == nil {
				existingServices[service] = map[string]string{}
			}

			host, _, err := net.SplitHostPort(v)
			if err != nil {
				return nil, err
			}

			existingServices[service][k] = host
		}
	}

	return existingServices, nil
}

// SuppotsLocalPool checks if the SystemInformation supports a MicroCloud configured local storage pool.
// Additionally returns whether such a pool already exists.
func (s *SystemInformation) SupportsLocalPool() (hasPool bool, supportsPool bool) {
	if s.existingLocalPool == nil {
		return false, true
	}
	if s.existingLocalPool.Driver == "zfs" && s.existingLocalPool.Status == "Created" {
		return true, true
	}

	return true, false
}

// SuppotsRemotePool checks if the SystemInformation supports a MicroCloud configured remote storage pool.
// Additionally returns whether such a pool already exists.
func (s *SystemInformation) SupportsRemotePool() (hasPool bool, supportsPool bool) {
	if s.existingRemotePool == nil {
		return false, true
	}

	if s.existingRemotePool.Driver == "ceph" && s.existingRemotePool.Status == "Created" {
		return true, true
	}

	return true, false
}

// SuppotsRemoteFSPool checks if the SystemInformation supports a MicroCloud configured remote-fs storage pool.
// Additionally returns whether such a pool already exists.
func (s *SystemInformation) SupportsRemoteFSPool() (hasPool bool, supportsPool bool) {
	if s.existingRemoteFSPool == nil {
		return false, true
	}

	if s.existingRemoteFSPool.Driver == "cephfs" && s.existingRemoteFSPool.Status == "Created" {
		return true, true
	}

	return true, false
}

func (s *SystemInformation) SupportsOVNNetwork() (hasNet bool, supportsNet bool) {
	if s.existingOVNNetwork == nil && s.existingUplinkNetwork == nil {
		return false, true
	}

	if s.existingOVNNetwork.Type == "ovn" && s.existingOVNNetwork.Status == "Created" && s.existingUplinkNetwork.Type == "physical" && s.existingUplinkNetwork.Status == "Created" {
		return true, true
	}

	return true, false
}

func (s *SystemInformation) SupportsFANNetwork() (hasNet bool, supportsNet bool) {
	if s.existingFanNetwork == nil {
		return false, true
	}

	if s.existingFanNetwork.Type == "bridge" && s.existingFanNetwork.Status == "Created" {
		return true, true
	}

	return true, false
}

func (s *SystemInformation) ServiceClustered(service types.ServiceType) bool {
	return len(s.ExistingServices[service]) > 0
}

// ClustersConflict returns whether
func ClustersConflict(systems map[string]*SystemInformation, services []types.ServiceType) (bool, types.ServiceType) {
	firstEncounteredClusters := map[types.ServiceType]map[string]string{}
	for _, info := range systems {
		for _, service := range services {
			// If a service is not clustered, it cannot conflict.
			if !info.ServiceClustered(service) {
				continue
			}

			// Record the first encountered cluster for each service.
			cluster, encountered := firstEncounteredClusters[service]
			if !encountered {
				firstEncounteredClusters[service] = info.ExistingServices[service]

				continue
			}

			// Check if the first encountered cluster for this service is identical to each system's record.
			for name, addr := range info.ExistingServices[service] {
				if cluster[name] != addr {
					return true, service
				}
			}

			if len(cluster) != len(info.ExistingServices[service]) {
				return true, service
			}
		}
	}

	return false, ""
}
