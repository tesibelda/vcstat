// vcstat package is a telegraf execd input plugin that gathers vCenter status and basic stats
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vcstat

import (
	"context"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/tesibelda/vcstat/pkg/vccollector"
)

type vcstatConfig struct {
	tls.ClientConfig
	VCenter  string          `toml:"vcenter"`
	Username string          `toml:"username"`
	Password string          `toml:"password"`
	Log      telegraf.Logger `toml:"-"`

	ClusterInstances   bool `toml:"cluster_instances"`
	DatastoreInstances bool `toml:"datastore_instances"`
	HostInstances      bool `toml:"host_instances"`
	HostHBAInstances   bool `toml:"host_hba_instances"`
	HostNICInstances   bool `toml:"host_nic_instances"`
	HostFwInstances    bool `toml:"host_firewall_instances"`
	NetDVSInstances    bool `toml:"net_dvs_instances"`
	NetDVPInstances    bool `toml:"net_dvp_instances"`

	ctx    context.Context
	cancel context.CancelFunc
	vcc    *vccollector.VcCollector
}

var sampleConfig = `
  ## vCenter URL to be monitored and its credential
  vcenter = "https://vcenter.local/sdk"
  username = "user@corp.local"
  password = "secret"

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
	inputs.Add("vcstat", func() telegraf.Input {
		return &vcstatConfig{
			VCenter:            "https://vcenter.local/sdk",
			Username:           "user@corp.local",
			Password:           "secret",
			ClusterInstances:   true,
			DatastoreInstances: false,
			HostInstances:      true,
			HostFwInstances:    false,
			HostHBAInstances:   false,
			HostNICInstances:   false,
			NetDVSInstances:    true,
			NetDVPInstances:    false,
		}
	})
}

// Init initializes internal vcstat variables with the provided configuration
func (vcs *vcstatConfig) Init() error {
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
	)

	return err
}

// Stop is called from telegraf core when a plugin is stopped and allows it to
// perform shutdown tasks.
func (vcs *vcstatConfig) Stop() {
	if vcs.vcc != nil {
		vcs.vcc.Close(vcs.ctx)
	}
	vcs.cancel()
}

// SampleConfig returns a set of default configuration to be used as a boilerplate when setting up
// Telegraf.
func (vcs *vcstatConfig) SampleConfig() string {
	return sampleConfig
}

// Description returns a short textual description of the plugin
func (vcs *vcstatConfig) Description() string {
	return "Gathers vSphere vCenter status and basic stats"
}

// Gather is the main data collection function called by the Telegraf core. It performs all
// the data collection and writes all metrics into the Accumulator passed as an argument.
func (vcs *vcstatConfig) Gather(acc telegraf.Accumulator) error {
	var err error

	//--- re-Init if needed
	if vcs.ctx == nil || vcs.ctx.Err() != nil || vcs.vcc == nil {
		if err = vcs.Init(); err != nil {
			return gatherError(acc, err)
		}
	}
	if !vcs.vcc.IsActive(vcs.ctx) {
		if err = vcs.vcc.Open(vcs.ctx); err != nil {
			return gatherError(acc, err)
		}
	}

	//--- Get vCenter basic stats
	if err = vcs.vcc.CollectVcenterInfo(vcs.ctx, acc); err != nil {
		return gatherError(acc, err)
	}

	//--- Get Datacenters info
	if err = vcs.vcc.CollectDatacenterInfo(vcs.ctx, acc); err != nil {
		return gatherError(acc, err)
	}

	//--- Get Clusters info
	if vcs.ClusterInstances {
		if err = vcs.vcc.CollectClusterInfo(vcs.ctx, acc); err != nil {
			return gatherError(acc, err)
		}
	}

	//--- Get Hosts, Network,... info
	if err = vcs.gatherHost(acc); err != nil {
		return gatherError(acc, err)
	}
	if err = vcs.gatherNetwork(acc); err != nil {
		return gatherError(acc, err)
	}

	//--- Get Datastores info
	if vcs.DatastoreInstances {
		if err = vcs.vcc.CollectDatastoresInfo(vcs.ctx, acc); err != nil {
			return gatherError(acc, err)
		}
	}

	return nil
}

// gatherHost gathers info and stats per host
func (vcs *vcstatConfig) gatherHost(acc telegraf.Accumulator) error {
	var err error

	if vcs.HostInstances {
		if err = vcs.vcc.CollectHostInfo(vcs.ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostHBAInstances {
		if err = vcs.vcc.CollectHostHBA(vcs.ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostNICInstances {
		if err = vcs.vcc.CollectHostNIC(vcs.ctx, acc); err != nil {
			return err
		}
	}

	if vcs.HostFwInstances {
		if err = vcs.vcc.CollectHostFw(vcs.ctx, acc); err != nil {
			return err
		}
	}

	return nil
}

// gatherNetwork gathers network entities info
func (vcs *vcstatConfig) gatherNetwork(acc telegraf.Accumulator) error {
	var err error

	if vcs.NetDVSInstances {
		if err = vcs.vcc.CollectNetDVS(vcs.ctx, acc); err != nil {
			return err
		}
	}

	if vcs.NetDVPInstances {
		if err = vcs.vcc.CollectNetDVP(vcs.ctx, acc); err != nil {
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
