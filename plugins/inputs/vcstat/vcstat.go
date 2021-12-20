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
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25/soap"
)

type VcStat struct {
	VCenter            string `toml:"vcenter"`
	Username           string `toml:"username"`
	Password           string `toml:"password"`
	InsecureSkipVerify bool   `toml:"insecure_skip_verify"`
	ClusterInstances   bool   `toml:"cluster_instances"`
	HostInstances      bool   `toml:"host_instances"`
	HostHBAInstances   bool   `toml:"host_hba_instances"`
	HostNICInstances   bool   `toml:"host_nic_instances"`
	NetDVSInstances    bool   `toml:"net_dvs_instances"`
	ctx                context.Context
	cancel             context.CancelFunc
	vccache            *cache.Session

	Log telegraf.Logger `toml:"-"`
}

func init() {
	inputs.Add("vcstat", func() telegraf.Input {
		return &VcStat{
			VCenter:            "https://vcenter.local/sdk",
			Username:           "user@corp.local",
			Password:           "secret",
			InsecureSkipVerify: true,
			ClusterInstances:   true,
			HostInstances:      true,
			HostHBAInstances:   false,
			HostNICInstances:   false,
			NetDVSInstances:    true,
		}
	})
}

func (vcs *VcStat) Init() error {
	vcs.ctx, vcs.cancel = context.WithCancel(context.Background())

	// Create a vSphere vCenter client
	u, err := soap.ParseURL(vcs.VCenter)
	if err != nil {
		return fmt.Errorf("Error parsing url for vcenter: %w", err)
	}
	if u==nil {
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
func (vcs *VcStat) Stop() {
	vcs.cancel()
}

// SampleConfig returns a set of default configuration to be used as a boilerplate when setting up
// Telegraf.
func (vcs *VcStat) SampleConfig() string {
	return `
  vcenter = "https://vcenter.local/sdk"
  username = "user@corp.local"
  password = "secret"
  insecure_skip_verify = false
  cluster_instances = true
  host_instances = true
  net_dvs_instances = false
`
}

// Description returns a short textual description of the plugin
func (vcs *VcStat) Description() string {
	return "Gathers vSphere vCenter status and basic stats"
}

// Gather is the main data collection function called by the Telegraf core. It performs all
// the data collection and writes all metrics into the Accumulator passed as an argument.
func (vcs *VcStat) Gather(acc telegraf.Accumulator) error {
	var err error = nil

	//--- re-Init if needed
	if vcs.ctx == nil || vcs.ctx.Err()!=nil || vcs.vccache == nil || vcs.vccache.URL == nil {
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
	if !cli.IsVC() {
		return gatherError(acc, fmt.Errorf("Error endpoint does not look like a vCenter"))
	}
	defer cli.Logout(vcs.ctx)
	c := cli.Client

	//--- Get vCenter basic stats
	vcC, err := NewVCCollector()
	err = vcC.Collect(vcs.ctx, c, acc)
	if err != nil && err != context.Canceled {
		// No need to signal errors if we were merely canceled.
		return gatherError(acc, err)
	}

	//--- Get Datacenters info and discovery of Dc instances
	dcC, err := NewDCCollector()
	err = dcC.Discover(vcs.ctx, c, vcC.dcs)
	if err != nil && err != context.Canceled {
		return gatherError(acc, err)
	}
	err = dcC.Collect(vcs.ctx, c, vcC.dcs, acc)
	if err != nil && err != context.Canceled {
		return gatherError(acc, err)
	}
	if err == context.Canceled {
		return nil
	}

	//--- Get Clusters info
	if vcs.ClusterInstances && len(dcC.clusters) > 0 {
		clC, err := NewClusterCollector()
		err = clC.Collect(vcs.ctx, c, vcC.dcs, dcC.clusters, acc)
		if err != nil && err != context.Canceled {
			return gatherError(acc, err)
		}
	}

	//--- Get Hosts info and host devices (hba,nic)
	if len(dcC.hosts) > 0 {
		hsC, err := NewHostCollector()
		if vcs.HostInstances {
			err = hsC.Collect(vcs.ctx, c, vcC.dcs, dcC.hosts, acc)
			if err != nil && err != context.Canceled {
				return gatherError(acc, err)
			}
			if err == context.Canceled {
				return nil
			}
		}

		//--- Get Host HBAs info
		if vcs.HostHBAInstances && bool(hsC) {
			err = hsC.CollectHBA(vcs.ctx, c, vcC.dcs, dcC.hosts, acc)
			if err != nil && err != context.Canceled {
				return gatherError(acc, err)
			}
		}

		//--- Get Host NICs info
		if vcs.HostNICInstances && bool(hsC) {
			err = hsC.CollectNIC(vcs.ctx, c, vcC.dcs, dcC.hosts, acc)
			if err != nil && err != context.Canceled {
				return gatherError(acc, err)
			}
		}
	}

	//--- Get Network info (Distributed Virtual Switchs at the moment)
	if vcs.NetDVSInstances && len(dcC.net) > 0 {
		ntC, err := NewNetCollector()
		err = ntC.CollectDvs(vcs.ctx, c, vcC.dcs, dcC.net, acc)
		if err != nil && err != context.Canceled {
			return gatherError(acc, err)
		}
	}

	return nil
}

// gatherError returns the error and adds it to the telegraf accumulator
func gatherError(acc telegraf.Accumulator, err error) error {
	if err != nil {
		acc.AddError(err)
	}
	return err
}
