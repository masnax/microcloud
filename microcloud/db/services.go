package db

//go:generate -command mapper lxd-generate db mapper -t services.mapper.go
//go:generate mapper reset
//
//go:generate mapper stmt -d github.com/canonical/microcluster/cluster -e Service objects
//go:generate mapper stmt -d github.com/canonical/microcluster/cluster -e Service objects-by-Name
//go:generate mapper stmt -d github.com/canonical/microcluster/cluster -e Service id
//go:generate mapper stmt -d github.com/canonical/microcluster/cluster -e Service create
//go:generate mapper stmt -d github.com/canonical/microcluster/cluster -e Service delete-by-Name
//go:generate mapper stmt -d github.com/canonical/microcluster/cluster -e Service update
//
//go:generate mapper method -d github.com/canonical/microcluster/cluster -e Service GetMany
//go:generate mapper method -d github.com/canonical/microcluster/cluster -e Service GetOne
//go:generate mapper method -d github.com/canonical/microcluster/cluster -e Service ID
//go:generate mapper method -d github.com/canonical/microcluster/cluster -e Service Exists
//go:generate mapper method -d github.com/canonical/microcluster/cluster -e Service Create
//go:generate mapper method -d github.com/canonical/microcluster/cluster -e Service DeleteOne-by-Name
//go:generate mapper method -d github.com/canonical/microcluster/cluster -e Service Update

// ServiceType represents supported services.
type ServiceType string

const (
	// MicroCloud represents a MicroCloud service.
	MicroCloud ServiceType = "MicroCloud"

	// MicroCeph represents a MicroCeph service.
	MicroCeph ServiceType = "MicroCeph"

	// LXD represents a LXD service.
	LXD ServiceType = "LXD"
)

// Service represents information about a supported service (LXD, MicroCeph).
type Service struct {
	ID       int
	Name     ServiceType `db:"primary=yes"`
	StateDir string
}

// ServiceFilter allows filtering on fields for the Service struct.
type ServiceFilter struct {
	Name *ServiceType
}
