// vcstat package is a telegraf execd input plugin so you can monitor vCenter status and basic stats
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vcstat

import (
	"context"
	"fmt"
	"net/url"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

type vcstatConfig struct {
	VCenter            string `toml:"vcenter"`
	Username           string `toml:"username"`
	Password           string `toml:"password"`
	InsecureSkipVerify bool   `toml:"insecure_skip_verify"`
	ClusterInstances   bool   `toml:"cluster_instances"`
	HostInstances      bool   `toml:"host_instances"`
	HostHBAInstances   bool   `toml:"host_hba_instances"`
	HostNICInstances   bool   `toml:"host_nic_instances"`
	HostFwInstances    bool   `toml:"host_firewall_instances"`
	NetDVSInstances    bool   `toml:"net_dvs_instances"`
	NetDVPInstances    bool   `toml:"net_dvp_instances"`
	ctx                context.Context
	cancel             context.CancelFunc
	vccache            *cache.Session

	Log telegraf.Logger `toml:"-"`
}

var sampleConfig = `
  ## vCenter URL to be monitored and its credential
  vcenter = "https://vcenter.local/sdk"
  username = "user@corp.local"
  password = "secret"
  ## Use SSL but skip chain & host verification
  insecure_skip_verify = false

  #### you may enable or disable data collection per instance type ####
  ## collect cluster measurements (vcstat_cluster)
  # cluster_instances = true
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
			InsecureSkipVerify: true,
			ClusterInstances:   true,
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
	vcs.ctx, vcs.cancel = context.WithCancel(context.Background())

	// Create a vSphere vCenter client
	u, err := soap.ParseURL(vcs.VCenter)
	if err != nil {
		return fmt.Errorf("Error parsing url for vcenter: %w", err)
	}
	if u == nil {
		return fmt.Errorf("Error parsing url for vcenter: returned nil")
	}
	u.User = url.UserPassword(vcs.Username, vcs.Password)

	// Share govc's session cache
	vcs.vccache = &cache.Session{
		URL:      u,
		Insecure: vcs.InsecureSkipVerify,
	}

	return nil
}

// Stop is called from telegraf core when a plugin is stopped and allows it to
// perform shutdown tasks.
func (vcs *vcstatConfig) Stop() {
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
	if vcs.ctx == nil || vcs.ctx.Err() != nil || vcs.vccache == nil || vcs.vccache.URL == nil {
		err = vcs.Init()
		if err != nil {
			return gatherError(acc, err)
		}
	}

	//--- Connect to vCenter API
	cli, err := govmomi.NewClient(vcs.ctx, vcs.vccache.URL, true)
	if err != nil {
		return gatherError(acc, err)
	}
	defer cli.Logout(vcs.ctx) //nolint   //no need for logout error checking
	if !cli.IsVC() {
		return gatherError(acc, fmt.Errorf("Error endpoint does not look like a vCenter"))
	}

	//--- Get vCenter basic stats
	vcC, _ := NewVCCollector() //err is always nil here
	err = vcC.Collect(vcs.ctx, cli.Client, acc)
	if err != nil {
		return gatherError(acc, err)
	}

	//--- Get Datacenters info
	dcC, _ := NewDCCollector(vcC.dcs) //err is always nil here
	err = dcC.Collect(vcs.ctx, cli.Client, acc)
	if err != nil {
		return gatherError(acc, err)
	}

	//--- Get Clusters info
	if vcs.ClusterInstances && len(dcC.clusters) > 0 {
		err = collectCluster(vcs.ctx, cli.Client, vcC.dcs, dcC.clusters, acc)
		if err != nil {
			return gatherError(acc, err)
		}
	}

	//--- Get Hosts, Network,... info
	err = vcs.gatherHost(cli.Client, vcC.dcs, dcC.hosts, acc)
	if err != nil {
		return gatherError(acc, err)
	}
	err = vcs.gatherNetwork(cli.Client, vcC.dcs, dcC.nets, acc)
	if err != nil {
		return gatherError(acc, err)
	}

	return nil
}

// gatherHost gathers info and stats per host
func (vcs *vcstatConfig) gatherHost(
	client *vim25.Client,
	dcs []*object.Datacenter,
	hsMap map[int][]*object.HostSystem,
	acc telegraf.Accumulator,
) error {
	var err error

	if vcs.HostInstances {
		err = collectHostInfo(vcs.ctx, client, dcs, hsMap, acc)
		if err != nil {
			return err
		}
	}

	if vcs.HostHBAInstances {
		err = collectHostHBA(vcs.ctx, client, dcs, hsMap, acc)
		if err != nil {
			return err
		}
	}

	if vcs.HostNICInstances {
		err = collectHostNIC(vcs.ctx, client, dcs, hsMap, acc)
		if err != nil {
			return err
		}
	}

	if vcs.HostFwInstances {
		err = collectHostFw(vcs.ctx, client, dcs, hsMap, acc)
		if err != nil {
			return err
		}
	}

	return nil
}

// gatherNetwork gathers network instances info
func (vcs *vcstatConfig) gatherNetwork(
	client *vim25.Client,
	dcs []*object.Datacenter,
	netMap map[int][]object.NetworkReference,
	acc telegraf.Accumulator,
) error {
	var err error

	if vcs.NetDVSInstances {
		err = collectNetDVS(vcs.ctx, client, dcs, netMap, acc)
		if err != nil {
			return err
		}
	}

	if vcs.NetDVPInstances {
		err = collectNetDVP(vcs.ctx, client, dcs, netMap, acc)
		if err != nil {
			return err
		}
	}

	return nil
}

// gatherError returns the error and adds it to the telegraf accumulator
func gatherError(acc telegraf.Accumulator, err error) error {
	// No need to signal errors if we were merely canceled.
	if err == context.Canceled {
		return nil
	}
	if err != nil {
		acc.AddError(err)
	}
	return err
}
