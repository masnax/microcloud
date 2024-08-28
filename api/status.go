package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/util"
	lxdAPI "github.com/canonical/lxd/shared/api"
	cephTypes "github.com/canonical/microceph/microceph/api/types"
	cephClient "github.com/canonical/microceph/microceph/client"
	microClient "github.com/canonical/microcluster/v2/client"
	"github.com/canonical/microcluster/v2/rest"
	microTypes "github.com/canonical/microcluster/v2/rest/types"
	"github.com/canonical/microcluster/v2/state"
	ovnTypes "github.com/canonical/microovn/microovn/api/types"
	ovnClient "github.com/canonical/microovn/microovn/client"

	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/service"
)

// StatusCmd represents the /1.0/status API on MicroCloud.
var StatusCmd = func(sh *service.Handler) rest.Endpoint {
	return rest.Endpoint{
		AllowedBeforeInit: true,
		Name:              "status",
		Path:              "status",

		Get: rest.EndpointAction{Handler: statusGet(sh), ProxyTarget: true},
	}
}

func statusGet(sh *service.Handler) endpointHandler {
	return func(s state.State, r *http.Request) response.Response {
		statuses := []types.Status{}

		if !microClient.IsNotification(r) {
			cluster, err := s.Cluster(true)
			if err != nil {
				return response.SmartError(err)
			}

			err = cluster.Query(r.Context(), true, func(ctx context.Context, c *microClient.Client) error {
				memberStatuses, err := client.GetStatus(ctx, c)
				if err != nil {
					return err
				}

				statuses = append(statuses, memberStatuses...)

				return nil
			})
			if err != nil {
				return response.SmartError(err)
			}
		}

		status := types.Status{
			Name:         sh.Name,
			Address:      sh.Address,
			Clusters:     map[types.ServiceType][]microTypes.ClusterMember{},
			OSDs:         []cephTypes.Disk{},
			CephServices: []cephTypes.Service{},
			OVNServices:  []ovnTypes.Service{},
		}

		err := sh.RunConcurrent("", "", func(s service.Service) error {
			var err error
			var microClient *microClient.Client
			var lxd lxd.InstanceServer
			switch s.Type() {
			case types.LXD:
				lxd, err = s.(*service.LXDService).Client(context.Background())
			case types.MicroCeph:
				microClient, err = s.(*service.CephService).Client("")
			case types.MicroOVN:
				microClient, err = s.(*service.OVNService).Client()
			case types.MicroCloud:
				microClient, err = s.(*service.CloudService).Client()
			}
			if err != nil {
				return err
			}

			if microClient != nil {
				clusterMembers, err := microClient.GetClusterMembers(context.Background())
				if err != nil && !lxdAPI.StatusErrorCheck(err, http.StatusServiceUnavailable) {
					return err
				}

				status.Clusters[s.Type()] = clusterMembers

				if s.Type() == types.MicroCeph {
					disks, err := cephClient.GetDisks(r.Context(), microClient)
					if err != nil {
						return err
					}

					for _, disk := range disks {
						if disk.Location == s.Name() {
							status.OSDs = append(status.OSDs, disk)
						}
					}

					services, err := cephClient.GetServices(r.Context(), microClient)
					if err != nil {
						return err
					}

					for _, service := range services {
						if service.Location == s.Name() {
							status.CephServices = append(status.CephServices, service)
						}
					}
				}

				if s.Type() == types.MicroOVN {
					services, err := ovnClient.GetServices(r.Context(), microClient)
					if err != nil {
						return err
					}

					for _, service := range services {
						if service.Location == s.Name() {
							status.OVNServices = append(status.OVNServices, service)
						}
					}
				}

			} else if lxd != nil {
				server, _, err := lxd.GetServer()
				if err != nil {
					return err
				}

				if server.Environment.ServerClustered {
					clusterMembers, err := lxd.GetClusterMembers()
					if err != nil {
						return err
					}

					certs, err := lxd.GetCertificates()
					if err != nil {
						return err
					}

					microMembers := make([]microTypes.ClusterMember, 0, len(clusterMembers))
					for _, member := range clusterMembers {
						url, err := url.Parse(member.URL)
						if err != nil {
							return err
						}

						addrPort, err := microTypes.ParseAddrPort(util.CanonicalNetworkAddress(url.Host, 8443))
						if err != nil {
							return err
						}

						// Microcluster requires a certificate to be specified in types.ClusterMemberLocal.
						var serverCert *microTypes.X509Certificate
						for _, cert := range certs {
							if cert.Type == "server" && cert.Name == member.ServerName {
								serverCert, err = microTypes.ParseX509Certificate(cert.Certificate)
								if err != nil {
									return err
								}
							}
						}

						microMember := microTypes.ClusterMember{
							ClusterMemberLocal: microTypes.ClusterMemberLocal{
								Name:        member.ServerName,
								Address:     addrPort,
								Certificate: *serverCert,
							},
							Role:       strings.Join(member.Roles, ","),
							Status:     microTypes.MemberStatus(member.Status),
							Extensions: []string{},
						}

						// If the status is Online, use the microcluster representation, all other cluster states will be considered invalid and be treated like an offline state.
						if member.Status == "Online" {
							microMember.Status = microTypes.MemberOnline
						}

						microMembers = append(microMembers, microMember)
					}

					status.Clusters[s.Type()] = microMembers
				}

			}

			return nil
		})
		if err != nil {
			return response.SmartError(err)
		}

		statuses = append(statuses, status)

		return response.SyncResponse(true, statuses)
	}
}
