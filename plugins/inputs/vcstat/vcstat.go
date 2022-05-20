// vcstat package is a telegraf execd input plugin that gathers vCenter status and basic stats
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

	"github.com/tesibelda/vcstat/pkg/vccollector"
)

type VCstatConfig struct {
	tls.ClientConfig
	VCenter             string `toml:"vcenter"`
	Username            string `toml:"username"`
	Password            string `toml:"password"`
	Timeout             config.Duration
	IntSkipNotRespondig int16           `toml:"intervals_skip_notresponding_esxcli_hosts"`
	Log                 telegraf.Logger `toml:"-"`

	ClusterInstances   bool `toml:"cluster_instances"`
	DatastoreInstances bool `toml:"datastore_instances"`
	HostInstances      bool `toml:"host_instances"`
	HostHBAInstances   bool `toml:"host_hba_instances"`
	HostNICInstances   bool `toml:"host_nic_instances"`
	HostFwInstances    bool `toml:"host_firewall_instances"`
	NetDVSInstances    bool `toml:"net_dvs_instances"`
	NetDVPInstances    bool `toml:"net_dvp_instances"`

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
  ## number of intervals to skip esxcli commands for not responding hosts
  # intervals_skip_notresponding_esxcli_hosts = 20

  ## Optional SSL Config
  # tls_ca = "/path/to/cafile"
  ## Use SSL but skip chain & host verification
  # insecure_skip_verify = false

  #### you may enable or disable data collection per instance type ####
  ## collect cluster measurements (vcstat_cluster)
  # cluster_instances = true
  ## collect datastore measurement (vcstat_datastore)
  # datastore_instances = false
  ## collect host status measurements (vcstat_host)
  # host_instances = true
  ## collect host firewall measurement (vcstat_host_firewall)
  # host_firewall_instances = false
  ## collect host bus adapter measurements (vcstat_host_hba)
  # host_hba_instances = false
  ## collect host network interface measurements (vcstat_host_nic)
  # host_nic_instances = false
  ## collect network distributed virtual switch measurements (vcstat_net_dvs)
  # net_dvs_instances = true
  ## collect network distributed virtual portgroup measurements (vcstat_net_dvp)
  # net_dvp_instances = false
`

func init() {
	m, _ := time.ParseDuration("60s") //nolint: hardcoded 1m expects no error
	inputs.Add("vcstat", func() telegraf.Input {
		return &VCstatConfig{
			VCenter:             "https://vcenter.local/sdk",
			Username:            "user@corp.local",
			Password:            "secret",
			Timeout:             config.Duration(time.Second * 0),
			IntSkipNotRespondig: 20,
			ClusterInstances:    true,
			DatastoreInstances:  false,
			HostInstances:       true,
			HostFwInstances:     false,
			HostHBAInstances:    false,
			HostNICInstances:    false,
			NetDVSInstances:     true,
			NetDVPInstances:     false,
			pollInterval:        m,
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
	vcs.vcc, err = vccollector.NewVCCollector(
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

	// Set vccollector dataduration as half of the telegraf shim polling interval
	_ = vcs.vcc.SetDataDuration(time.Duration(vcs.pollInterval.Seconds() / 2))
	// Set vccollector duration to skip not responding hosts to esxcli commands
	_ = vcs.vcc.SetSkipHostNotRespondingDuration(
		time.Duration(vcs.pollInterval.Seconds() * float64(vcs.IntSkipNotRespondig)),
	)

	// selfmonitoring
	u, err := url.Parse(vcs.VCenter)
	if err != nil {
		return fmt.Errorf("Error parsing URL for vcenter: %w", err)
	}
	tags := map[string]string{
		"vcenter": u.Hostname(),
	}
	vcs.GatherTime = selfstat.Register("vcstat", "gather_time_ns", tags)
	vcs.NotRespondingHosts = selfstat.Register("vcstat", "notresponding_esxcli_hosts", tags)
	vcs.SessionsCreated = selfstat.Register("vcstat", "sessions_created", tags)

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

// SampleConfig returns a set of default configuration to be used as a boilerplate when setting up
// Telegraf.
func (vcs *VCstatConfig) SampleConfig() string {
	return sampleConfig
}

// Description returns a short textual description of the plugin
func (vcs *VCstatConfig) Description() string {
	return "Gathers vSphere vCenter status and basic stats"
}

// Gather is the main data collection function called by the Telegraf core. It performs all
// the data collection and writes all metrics into the Accumulator passed as an argument.
func (vcs *VCstatConfig) Gather(acc telegraf.Accumulator) error {
	var (
		col       *vccollector.VcCollector
		startTime time.Time
		err       error
	)

	//--- re-Init if needed
	if vcs.ctx == nil || vcs.ctx.Err() != nil || vcs.vcc == nil {
		if err = vcs.Init(); err != nil {
			return gatherError(acc, err)
		}
	}
	col = vcs.vcc
	if !col.IsActive(vcs.ctx) {
		if vcs.SessionsCreated.Get() > 0 {
			acc.AddError(fmt.Errorf("vCenter session not active, re-authenticating..."))
		}
		if err = col.Open(vcs.ctx, time.Duration(vcs.Timeout)); err != nil {
			return gatherError(acc, err)
		}
		vcs.SessionsCreated.Incr(1)
	}
	acc.SetPrecision(getPrecision(vcs.pollInterval))
	// poll using a context with timeout
	ctx1, cancel1 := context.WithTimeout(vcs.ctx, time.Duration(vcs.Timeout))
	defer cancel1()
	startTime = time.Now()

	//--- Get vCenter basic stats
	if err = col.CollectVcenterInfo(ctx1, acc); err != nil {
		return gatherError(acc, err)
	}

	//--- Get Datacenters info
	if vcs.ClusterInstances || vcs.HostInstances {
		if err = col.CollectDatacenterInfo(ctx1, acc); err != nil {
			return gatherError(acc, err)
		}
	}

	//--- Get Clusters info
	if vcs.ClusterInstances {
		if err = col.CollectClusterInfo(ctx1, acc); err != nil {
			return gatherError(acc, err)
		}
	}

	//--- Get Hosts, Network,... info
	if err = vcs.gatherHost(ctx1, acc); err != nil {
		return gatherError(acc, err)
	}
	if err = vcs.gatherNetwork(ctx1, acc); err != nil {
		return gatherError(acc, err)
	}

	//--- Get Datastores info
	if vcs.DatastoreInstances {
		if err = col.CollectDatastoresInfo(ctx1, acc); err != nil {
			return gatherError(acc, err)
		}
	}

	// selfmonitoring
	vcs.GatherTime.Set(int64(time.Since(startTime).Nanoseconds()))
	vcs.NotRespondingHosts.Set(int64(vcs.vcc.GetNumberNotRespondingHosts()))
	for _, m := range selfstat.Metrics() {
		if m.Name() != "internal_agent" {
			acc.AddMetric(m)
		}
	}

	return nil
}

// gatherHost gathers info and stats per host
func (vcs *VCstatConfig) gatherHost(ctx context.Context, acc telegraf.Accumulator) error {
	var (
		col *vccollector.VcCollector
		err error
	)

	col = vcs.vcc
	if vcs.HostInstances {
		if err = col.CollectHostInfo(ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostHBAInstances {
		if err = col.CollectHostHBA(ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostNICInstances {
		if err = col.CollectHostNIC(ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostFwInstances {
		if err = col.CollectHostFw(ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostHBAInstances || vcs.HostNICInstances || vcs.HostFwInstances {
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

// gatherError adds the error to the telegraf accumulator
func gatherError(acc telegraf.Accumulator, err error) error {
	// No need to signal errors if we were merely canceled.
	if err == context.Canceled {
		return nil
	}
	acc.AddError(err)
	return nil
}

// Returns the rounding precision for metrics
func getPrecision(interval time.Duration) time.Duration {
	switch {
	case interval >= time.Second:
		return time.Second
	case interval >= time.Millisecond:
		return time.Millisecond
	case interval >= time.Microsecond:
		return time.Microsecond
	default:
		return time.Nanosecond
	}
}