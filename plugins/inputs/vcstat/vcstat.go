// vcstat is a VMware vSphere input plugin for Telegraf that gathers status and basic
//  stats from vCenter
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vcstat

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/selfstat"

	"github.com/tesibelda/vcstat/internal/vccollector"
	"github.com/tesibelda/vcstat/pkg/tgplus"
)

type VCstatConfig struct {
	tls.ClientConfig
	VCenter             string `toml:"vcenter"`
	Username            string `toml:"username"`
	Password            string `toml:"password"`
	InternalAlias       string `toml:"internal_alias"`
	Timeout             config.Duration
	IntSkipNotRespondig int16           `toml:"intervals_skip_notresponding_esxcli_hosts"`
	QueryBulkSize       int             `toml:"query_bulk_size"`
	Log                 telegraf.Logger `toml:"-"`

	ClustersExclude []string `toml:"clusters_exclude"`
	ClustersInclude []string `toml:"clusters_include"`
	HostsExclude    []string `toml:"hosts_exclude"`
	HostsInclude    []string `toml:"hosts_include"`
	VmsExclude      []string `toml:"vms_exclude"`
	VmsInclude      []string `toml:"vms_include"`

	ClusterInstances   bool `toml:"cluster_instances"`
	DatastoreInstances bool `toml:"datastore_instances"`
	HostInstances      bool `toml:"host_instances"`
	HostHBAInstances   bool `toml:"host_hba_instances"`
	HostNICInstances   bool `toml:"host_nic_instances"`
	HostFwInstances    bool `toml:"host_firewall_instances"`
	HostGraphics       bool `toml:"host_graphics_instances"`
	HostServices       bool `toml:"host_service_instances"`
	NetDVSInstances    bool `toml:"net_dvs_instances"`
	NetDVPInstances    bool `toml:"net_dvp_instances"`
	VMInstances        bool `toml:"vm_instances"`

	version      string
	pollInterval time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	vcc          *vccollector.VcCollector

	GatherTime         selfstat.Stat
	NotRespondingHosts selfstat.Stat
	SessionsCreated    selfstat.Stat
}

var sampleConfig = `
  ## vCenter URL to be monitored and its credential
  vcenter = "https://vcenter.local/sdk"
  username = "user@corp.local"
  password = "secret"
  ## requests timeout. Here 0s is interpreted as the polling interval
  # timeout = "0s"

  ## Optional SSL Config
  # tls_ca = "/path/to/cafile"
  ## Use SSL but skip chain & host verification
  # insecure_skip_verify = false

  ## optional alias tag for internal metrics
  # internal_alias = ""
  ## Max number of objects to gather per query
  # query_bulk_size = 100
  ## number of intervals to skip esxcli commands for not responding hosts
  # intervals_skip_notresponding_esxcli_hosts = 20

  ## Filter clusters by name, default is no filtering
  ## cluster names can be specified as glob patterns
  # clusters_include = []
  # clusters_exclude = []

  ## Filter hosts by name, default is no filtering
  ## host names can be specified as glob patterns
  # hosts_include = []
  # hosts_exclude = []

  ## Filter VMs by name, default is no filtering
  ## VM names can be specified as glob patterns
  # vms_include = []
  # vms_exclude = []

  #### you may enable or disable data collection per instance type ####
  ## collect cluster measurement (vcstat_cluster)
  # cluster_instances = true
  ## collect datastore measurement (vcstat_datastore)
  # datastore_instances = false
  ## collect host status measurement (vcstat_host)
  # host_instances = true
  ## collect host firewall measurement (vcstat_host_firewall)
  # host_firewall_instances = false
  ## collect host graphics measurement (vcstat_host_graphics)
  # host_graphics_instances = false
  ## collect host bus adapter measurement (vcstat_host_hba)
  # host_hba_instances = false
  ## collect host network interface measurement (vcstat_host_nic)
  # host_nic_instances = false
  ## collect host services measurement (vcstat_host_service)
  # host_service_instances = false
  ## collect network distributed virtual switch measurement (vcstat_net_dvs)
  # net_dvs_instances = true
  ## collect network distributed virtual portgroup measurement (vcstat_net_dvp)
  # net_dvp_instances = false
  ## collect virtual machine measurement (vcstat_vm)
  # vm_instances = false
`

func init() {
	inputs.Add("vcstat", func() telegraf.Input {
		return &VCstatConfig{
			VCenter:             "https://vcenter.local/sdk",
			Username:            "user@corp.local",
			Password:            "secret",
			InternalAlias:       "",
			Timeout:             config.Duration(time.Second * 0),
			QueryBulkSize:       100,
			IntSkipNotRespondig: 20,
			ClusterInstances:    true,
			DatastoreInstances:  false,
			HostInstances:       true,
			HostFwInstances:     false,
			HostGraphics:        false,
			HostServices:        false,
			HostHBAInstances:    false,
			HostNICInstances:    false,
			NetDVSInstances:     true,
			NetDVPInstances:     false,
			VMInstances:         false,
			pollInterval:        time.Second * 60,
		}
	})
}

// Init initializes internal vcstat variables with the provided configuration
func (vcs *VCstatConfig) Init() error {
	var err error

	vcs.ctx, vcs.cancel = context.WithCancel(context.Background())
	if vcs.vcc != nil {
		vcs.vcc.Close(vcs.ctx)
	}
	vcs.vcc, err = vccollector.New(
		vcs.ctx,
		vcs.VCenter,
		vcs.Username,
		vcs.Password,
		&vcs.ClientConfig,
		vcs.pollInterval,
	)
	if err != nil {
		return err
	}

	/// Set vccollector options
	vcs.vcc.SetDataDuration(time.Duration(vcs.pollInterval.Seconds() * 0.9))
	vcs.vcc.SetMaxResponseTime(time.Duration(vcs.pollInterval))
	vcs.vcc.SetSkipHostNotRespondingDuration(
		time.Duration(vcs.pollInterval.Seconds() * float64(vcs.IntSkipNotRespondig)),
	)
	vcs.vcc.SetQueryChunkSize(vcs.QueryBulkSize)
	err = vcs.vcc.SetFilterClusters(vcs.ClustersInclude, vcs.ClustersExclude)
	if err != nil {
		return fmt.Errorf("Error parsing clusters filters: %w", err)
	}
	err = vcs.vcc.SetFilterHosts(vcs.HostsInclude, vcs.HostsExclude)
	if err != nil {
		return fmt.Errorf("Error parsing hosts filters: %w", err)
	}
	err = vcs.vcc.SetFilterVms(vcs.VmsInclude, vcs.VmsExclude)
	if err != nil {
		return fmt.Errorf("Error parsing VMs filters: %w", err)
	}

	_, err = url.Parse(vcs.VCenter)
	if err != nil {
		return fmt.Errorf("Error parsing URL for vcenter: %w", err)
	}

	return err
}

// Stop is called from telegraf core when a plugin is stopped and allows it to
// perform shutdown tasks.
func (vcs *VCstatConfig) Stop() {
	if vcs.vcc != nil {
		vcs.vcc.Close(vcs.ctx)
	}
	vcs.cancel()
}

// SetPollInterval allows telegraf shim to tell vcstat the configured polling interval
func (vcs *VCstatConfig) SetPollInterval(pollInterval time.Duration) error {
	vcs.pollInterval = pollInterval
	if vcs.Timeout == 0 {
		vcs.Timeout = config.Duration(pollInterval)
	}
	return nil
}

// SetVersion let telegraf shim know this version
func (vcs *VCstatConfig) SetVersion(version string) {
	vcs.version = version
}

// StartSelfMetrics initialices selfmonitoring
func (vcs *VCstatConfig) StartSelfMetrics() {
	u, err := url.Parse(vcs.VCenter)
	if err != nil {
		return
	}
	tags := map[string]string{
		"alias":          vcs.InternalAlias,
		"vcenter":        u.Hostname(),
		"vcstat_version": vcs.version,
	}
	vcs.GatherTime = selfstat.Register("vcstat", "gather_time_ns", tags)
	vcs.NotRespondingHosts = selfstat.Register("vcstat", "notresponding_esxcli_hosts", tags)
	vcs.SessionsCreated = selfstat.Register("vcstat", "sessions_created", tags)
}

// SampleConfig returns a set of default configuration to be used as a boilerplate when setting up
// Telegraf.
func (vcs *VCstatConfig) SampleConfig() string {
	return sampleConfig
}

// Description returns a short textual description of the plugin
func (vcs *VCstatConfig) Description() string {
	return "Gathers status and basic stats from VMware vCenter"
}

// Gather is the main data collection function called by the Telegraf core. It performs all
// the data collection and writes all metrics into the Accumulator passed as an argument.
func (vcs *VCstatConfig) Gather(acc telegraf.Accumulator) error {
	var (
		startTime time.Time
		err       error
	)

	if err = vcs.keepActiveSession(acc); err != nil {
		return tgplus.GatherError(acc, err)
	}
	acc.SetPrecision(tgplus.GetPrecision(vcs.pollInterval))

	// poll using a context with timeout
	ctxT, cancelT := context.WithTimeout(vcs.ctx, time.Duration(vcs.Timeout))
	defer cancelT()
	startTime = time.Now()

	//--- Get vCenter, DCs and Clusters info
	if err = vcs.gatherHighLevelEntities(ctxT, acc); err != nil {
		return tgplus.GatherError(acc, err)
	}

	//--- Get Hosts, Networks and Storage info
	if err = vcs.gatherHost(ctxT, acc); err != nil {
		return tgplus.GatherError(acc, err)
	}
	if err = vcs.gatherNetwork(ctxT, acc); err != nil {
		return tgplus.GatherError(acc, err)
	}
	if err = vcs.gatherStorage(ctxT, acc); err != nil {
		return tgplus.GatherError(acc, err)
	}

	//--- Get VM info
	if err = vcs.gatherVM(ctxT, acc); err != nil {
		return tgplus.GatherError(acc, err)
	}

	// selfmonitoring
	vcs.GatherTime.Set(int64(time.Since(startTime).Nanoseconds()))
	if vcs.HostHBAInstances || vcs.HostNICInstances || vcs.HostFwInstances {
		vcs.NotRespondingHosts.Set(int64(vcs.vcc.GetNumberNotRespondingHosts()))
	}
	for _, m := range selfstat.Metrics() {
		if m.Name() != "internal_agent" {
			acc.AddMetric(m)
		}
	}

	return nil
}

// keepActiveSession keeps an active session with vsphere
func (vcs *VCstatConfig) keepActiveSession(acc telegraf.Accumulator) error {
	var (
		col *vccollector.VcCollector
		err error
	)

	if vcs.ctx == nil || vcs.ctx.Err() != nil || vcs.vcc == nil {
		if err = vcs.Init(); err != nil {
			return err
		}
	}
	col = vcs.vcc
	if !col.IsActive(vcs.ctx) {
		if vcs.SessionsCreated.Get() > 0 {
			acc.AddError(fmt.Errorf("vCenter session not active, re-authenticating..."))
		}
		if err = col.Open(vcs.ctx, time.Duration(vcs.Timeout)); err != nil {
			return err
		}
		vcs.SessionsCreated.Incr(1)
	}

	return nil
}

// gatherHighLevelEntities gathers datacenters and clusters stats
func (vcs *VCstatConfig) gatherHighLevelEntities(ctx context.Context, acc telegraf.Accumulator) error {
	var (
		col *vccollector.VcCollector
		err error
	)

	col = vcs.vcc

	//--- Get vCenter basic stats
	if err = col.CollectVcenterInfo(ctx, acc); err != nil {
		return err
	}

	//--- Get Datacenters info
	if vcs.ClusterInstances || vcs.HostInstances {
		if err = col.CollectDatacenterInfo(ctx, acc); err != nil {
			return err
		}
	}

	//--- Get Clusters info
	if vcs.ClusterInstances {
		if err = col.CollectClusterInfo(ctx, acc); err != nil {
			return err
		}
	}

	return nil
}

// gatherHost gathers info and stats per host
func (vcs *VCstatConfig) gatherHost(ctx context.Context, acc telegraf.Accumulator) error {
	var (
		col                 *vccollector.VcCollector
		hasEsxcliCollection bool
		err                 error
	)

	col = vcs.vcc
	if vcs.HostInstances {
		if err = col.CollectHostInfo(ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostHBAInstances {
		hasEsxcliCollection = true
		if err = col.CollectHostHBA(ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostNICInstances {
		hasEsxcliCollection = true
		if err = col.CollectHostNIC(ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostFwInstances {
		hasEsxcliCollection = true
		if err = col.CollectHostFw(ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostGraphics {
		hasEsxcliCollection = true
		if err = col.CollectHostGraphics(ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostServices {
		if err = col.CollectHostServices(ctx, acc); err != nil {
			return err
		}
	}

	if hasEsxcliCollection {
		if err = col.ReportHostEsxcliResponse(ctx, acc); err != nil {
			return err
		}
	}

	return nil
}

// gatherNetwork gathers network entities info
func (vcs *VCstatConfig) gatherNetwork(ctx context.Context, acc telegraf.Accumulator) error {
	var (
		col *vccollector.VcCollector
		err error
	)

	col = vcs.vcc
	if vcs.NetDVSInstances {
		if err = col.CollectNetDVS(ctx, acc); err != nil {
			return err
		}
	}

	if vcs.NetDVPInstances {
		if err = col.CollectNetDVP(ctx, acc); err != nil {
			return err
		}
	}

	return nil
}

// gatherStorage gathers storage entities info
func (vcs *VCstatConfig) gatherStorage(ctx context.Context, acc telegraf.Accumulator) error {
	if vcs.DatastoreInstances {
		var col *vccollector.VcCollector
		var err error
		col = vcs.vcc
		if col == nil {
			return nil
		}
		if err = col.CollectDatastoresInfo(ctx, acc); err != nil {
			return tgplus.GatherError(acc, err)
		}
	}

	return nil
}

// gatherVM gathers virtual machines info
func (vcs *VCstatConfig) gatherVM(ctx context.Context, acc telegraf.Accumulator) error {
	if vcs.VMInstances {
		var col *vccollector.VcCollector
		var err error
		col = vcs.vcc
		if col == nil {
			return nil
		}
		if err = col.CollectVmsInfo(ctx, acc); err != nil {
			return tgplus.GatherError(acc, err)
		}
	}

	return nil
}
