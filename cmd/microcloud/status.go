package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/canonical/microcluster/v2/microcluster"
	microTypes "github.com/canonical/microcluster/v2/rest/types"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/canonical/microcloud/microcloud/api"
	"github.com/canonical/microcloud/microcloud/api/types"
	"github.com/canonical/microcloud/microcloud/client"
	"github.com/canonical/microcloud/microcloud/cmd/style"
	"github.com/canonical/microcloud/microcloud/service"
)

// Warning represents a warning message with a severity level.
type Warning struct {
	Level   StatusLevel
	Message string
}

// Warnings is a list of warnings.
type Warnings []Warning

// Status returns the overall status of the warning list.
// If there are any Error level warnings, the status will be error.
// Otherwise, if there are any Warn level warnings, the status will be warn.
// Finally, the status will be Success, implying no warnings.
func (w Warnings) Status() StatusLevel {
	if len(w) == 0 {
		return Success
	}

	for _, warning := range w {
		if warning.Level == Error {
			return Error
		}
	}

	return Warn
}

// StatusLevel represents the severity level of warnings.
type StatusLevel int

const (
	// Success represents a lack of warnings.
	Success StatusLevel = iota

	// Warn represents a medium severity warning.
	Warn

	// Error represents a critical warning.
	Error
)

// Symbol returns the single-character symbol representing the StatusLevel, color coded.
func (s StatusLevel) Symbol() string {
	switch s {
	case Success:
		return style.SuccessSymbol()
	case Warn:
		return style.WarningSymbol()
	case Error:
		return style.ErrorSymbol()
	}

	return ""
}

// Symbol returns a word representing the StatusLevel, color coded.
func (s StatusLevel) String() string {
	switch s {
	case Success:
		return style.SuccessColor("HEALTHY")
	case Warn:
		return style.WarningColor("WARNING")
	case Error:
		return style.ErrorColor("ERROR")
	}

	return ""
}

type cmdStatus struct {
	common *CmdControl
}

func (c *cmdStatus) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Deployment status with configuration warnings",
		RunE:  c.Run,
	}

	return cmd
}

func (c *cmdStatus) Run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return cmd.Help()
	}

	cloudApp, err := microcluster.App(microcluster.Args{StateDir: c.common.FlagMicroCloudDir})
	if err != nil {
		return err
	}

	status, err := cloudApp.Status(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to get MicroCloud status: %w", err)
	}

	if !status.Ready {
		return fmt.Errorf("MicroCloud is uninitialized, run 'microcloud init' first")
	}

	cfg := initConfig{
		autoSetup: true,
		bootstrap: false,
		common:    c.common,
		asker:     &c.common.asker,
		systems:   map[string]InitSystem{},
		state:     map[string]service.SystemInformation{},
	}

	cfg.name = status.Name
	cfg.address = status.Address.Addr().String()

	services := []types.ServiceType{types.MicroCloud, types.LXD}
	optionalServices := map[types.ServiceType]string{
		types.MicroCeph: api.MicroCephDir,
		types.MicroOVN:  api.MicroOVNDir,
	}

	services, err = cfg.askMissingServices(services, optionalServices)
	if err != nil {
		return err
	}

	// Instantiate a handler for the services.
	sh, err := service.NewHandler(status.Name, status.Address.Addr().String(), c.common.FlagMicroCloudDir, c.common.FlagLogDebug, c.common.FlagLogVerbose, services...)
	if err != nil {
		return err
	}

	cloudClient, err := sh.Services[types.MicroCloud].(*service.CloudService).Client()
	if err != nil {
		return err
	}

	// Query the status API for the cluster.
	statuses, err := client.GetStatus(context.Background(), cloudClient)
	if err != nil {
		return err
	}

	// compile all warning messages.
	warnings := compileWarnings(cfg.name, statuses)

	// Print the warning summary, and all warnings.
	fmt.Println("")
	fmt.Printf(" %s: %s\n", style.TextColor("Status"), warnings.Status().String())
	fmt.Println("")
	for _, w := range warnings {
		fmt.Printf(" %s %s %s\n", style.TableBorderColor("┃"), w.Level.Symbol(), w.Message)
	}

	fmt.Println("")

	headers := []string{"Name", "Address", "OSD Disks", "MicroCeph Services", "MicroOVN Services", "Status"}

	var localStatus types.Status
	for _, s := range statuses {
		if s.Name == cfg.name {
			localStatus = s
		}
	}

	// Format and colorize cells of the table.
	rows := make([][]string, 0, len(statuses))
	for _, s := range statuses {
		rows = append(rows, formatStatusRow(localStatus, s))
	}

	t := table.New()
	t = t.Headers(headers...)
	t = t.Rows(rows...)
	t = t.Border(lipgloss.NormalBorder())
	t = t.BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color(style.DarkGrey)))
	t = t.StyleFunc(func(row, col int) lipgloss.Style {
		s := lipgloss.NewStyle()
		s = s.Padding(0, 1)

		if row == 0 {
			headers[col] = style.TableHeaderColor(headers[col])
		}

		return s
	})

	// Print the table.
	fmt.Println(t.String())

	return nil
}

// compileWarnings returns a set of warnings based on the given set of statuses. The name supplied should be the local cluster name.
func compileWarnings(name string, statuses []types.Status) Warnings {
	// Systems that exist in other clusters but not in MicroCloud.
	unmanagedSystems := map[types.ServiceType]map[string]bool{}

	// Systems that exist in MicroCloud, but not other clusters.
	orphanedSystems := map[types.ServiceType]map[string]bool{}

	// Services that are uninitialized on a system.
	uninstalledServices := map[types.ServiceType][]string{}

	// Services undergoing schema/API upgrades.
	upgradingServices := map[types.ServiceType]bool{}

	// Systems that are offline on at least one service.
	offlineSystems := map[string]bool{}

	osdsConfigured := false
	clusterSize := 0
	osdCount := 0

	for _, s := range statuses {
		if s.Name == name {
			clusterSize = len(s.Clusters[types.MicroCloud])
			for service, clusterMembers := range s.Clusters {
				for _, member := range clusterMembers {
					if member.Status == microTypes.MemberNeedsUpgrade || member.Status == microTypes.MemberUpgrading {
						upgradingServices[service] = true
					} else if member.Status != microTypes.MemberOnline {
						offlineSystems[member.Name] = true
					}
				}
			}
		}

		osdCount = osdCount + len(s.OSDs)
		allServices := []types.ServiceType{types.LXD, types.MicroCeph, types.MicroOVN, types.MicroCloud}
		cloudMembers := make(map[string]bool, len(s.Clusters[types.MicroCloud]))
		for _, member := range s.Clusters[types.MicroCloud] {
			cloudMembers[member.Name] = true
		}

		for _, service := range allServices {
			if len(s.Clusters[service]) == 0 {
				if uninstalledServices[service] == nil {
					uninstalledServices[service] = []string{}
				}
				uninstalledServices[service] = append(uninstalledServices[service], s.Name)
			}

			if service == types.MicroCloud || s.Name != name {
				continue
			}

			for _, member := range s.Clusters[service] {
				if !cloudMembers[member.Name] {
					if unmanagedSystems[service] == nil {
						unmanagedSystems[service] = map[string]bool{}
					}

					unmanagedSystems[service][member.Name] = true
				}
			}

			if len(s.Clusters[service]) > 0 {
				clusterMap := make(map[string]bool, len(s.Clusters[service]))
				for _, member := range s.Clusters[service] {
					clusterMap[member.Name] = true
				}

				for name := range cloudMembers {
					if !clusterMap[name] {
						if orphanedSystems[service] == nil {
							orphanedSystems[service] = map[string]bool{}
						}

						orphanedSystems[service][name] = true
					}
				}
			}
		}

		if osdCount > 0 && len(s.Clusters[types.MicroCeph]) > 0 {
			osdsConfigured = true
		}
	}

	// Format the actual warnings based on the collected data.
	warnings := Warnings{}
	if clusterSize < 3 {
		tmpl := style.Format{Color: style.White, Arg: "%s: %d systems are required for effective fault tolerance"}
		msg := style.ColorPrintf(tmpl,
			style.Format{Color: style.Red, Arg: "Reliability risk"},
			style.Format{Color: style.Purple, Arg: 3},
		)

		warnings = append(warnings, Warning{Level: Error, Message: msg})
	}

	if osdCount < 3 && osdsConfigured {
		tmpl := style.Format{Color: style.White, Arg: "%s: MicroCeph OSD replication recommends at least %d disks across %d systems"}
		msg := style.ColorPrintf(tmpl,
			style.Format{Color: style.Red, Arg: "Data loss risk"},
			style.Format{Color: style.Purple, Arg: 3},
			style.Format{Color: style.Purple, Arg: 3},
		)

		warnings = append(warnings, Warning{Level: Error, Message: msg})
	}

	if len(uninstalledServices[types.LXD]) > 0 {
		tmpl := style.Format{Color: style.White, Arg: "LXD is not installed on %s"}
		msg := style.ColorPrintf(tmpl, style.Format{Color: style.Purple, Arg: strings.Join(uninstalledServices[types.LXD], ", ")})
		warnings = append(warnings, Warning{Level: Error, Message: msg})
	}

	for service, systems := range orphanedSystems {
		list := make([]string, 0, len(systems))
		for name := range systems {
			list = append(list, name)
		}

		tmpl := style.Format{Color: style.White, Arg: "MicroCloud members not found in %s: %s"}
		msg := style.ColorPrintf(tmpl,
			style.Format{Color: style.Purple, Arg: service},
			style.Format{Color: style.Purple, Arg: strings.Join(list, ", ")})
		warnings = append(warnings, Warning{Level: Error, Message: msg})
	}

	if !osdsConfigured && len(uninstalledServices[types.MicroCeph]) < clusterSize {
		warnings = append(warnings, Warning{Level: Warn, Message: style.TextColor("No MicroCeph OSDs configured")})
	}

	for name := range offlineSystems {
		tmpl := style.Format{Color: style.White, Arg: "%s is not available"}
		msg := style.ColorPrintf(tmpl, style.Format{Color: style.Purple, Arg: name})
		warnings = append(warnings, Warning{Level: Warn, Message: msg})
	}

	for service := range upgradingServices {
		tmpl := style.Format{Color: style.White, Arg: "%s upgrade in progress"}
		msg := style.ColorPrintf(tmpl, style.Format{Color: style.Purple, Arg: service})
		warnings = append(warnings, Warning{Level: Warn, Message: msg})
	}

	for service, names := range uninstalledServices {
		if service == types.LXD || service == types.MicroCloud {
			continue
		}

		tmpl := style.Format{Color: style.White, Arg: "%s is not installed on %s"}
		msg := style.ColorPrintf(tmpl,
			style.Format{Color: style.Purple, Arg: service},
			style.Format{Color: style.Purple, Arg: strings.Join(names, ", ")})
		warnings = append(warnings, Warning{Level: Warn, Message: msg})
	}

	for service, systems := range unmanagedSystems {
		list := make([]string, 0, len(systems))
		for name := range systems {
			list = append(list, name)
		}

		tmpl := style.Format{Color: style.White, Arg: "Found %s systems not managed by MicroCloud: %s"}
		msg := style.ColorPrintf(tmpl,
			style.Format{Color: style.Purple, Arg: service},
			style.Format{Color: style.Purple, Arg: strings.Join(list, ",")})
		warnings = append(warnings, Warning{Level: Warn, Message: msg})
	}

	return warnings
}

// formatStatusRow formats the given status data for a cluster member into a row of the table.
// Also takes the local system's status which will be used as the source of truth for cluster member responsiveness.
func formatStatusRow(localStatus types.Status, s types.Status) []string {
	osds := style.WarningColor("0")
	if len(s.OSDs) > 0 {
		osds = style.TextColor(strconv.Itoa(len(s.OSDs)))
	}

	cephServices := style.WarningColor("-")

	if len(s.CephServices) > 0 {
		services := make([]string, 0, len(s.CephServices))
		for _, service := range s.CephServices {
			services = append(services, service.Service)
		}

		cephServices = style.TextColor(strings.Join(services, ","))
	}

	ovnServices := style.WarningColor("-")
	if len(s.OVNServices) > 0 {
		services := make([]string, 0, len(s.OVNServices))
		for _, service := range s.OVNServices {
			services = append(services, service.Service)
		}

		ovnServices = style.TextColor(strings.Join(services, ","))
	}

	if len(s.Clusters[types.MicroOVN]) == 0 {
		ovnServices = style.ErrorColor("-")
	}

	if len(s.Clusters[types.MicroCeph]) == 0 {
		cephServices = style.ErrorColor("-")
		osds = style.ErrorColor("-")
	}

	status := style.SuccessColor(string(microTypes.MemberOnline))
	for _, members := range localStatus.Clusters {
		for _, member := range members {
			if member.Name != s.Name {
				continue
			}

			// Only set the service status to upgrading if no other member has a more urgent status.
			if member.Status == microTypes.MemberUpgrading || member.Status == microTypes.MemberNeedsUpgrade {
				if status == style.SuccessColor(string(microTypes.MemberOnline)) {
					status = style.WarningColor(string(member.Status))
				}
			} else if member.Status != microTypes.MemberOnline {
				status = style.ErrorColor(string(member.Status))
			}
		}
	}

	return []string{style.TextColor(s.Name), style.TextColor(s.Address), osds, cephServices, ovnServices, status}
}
