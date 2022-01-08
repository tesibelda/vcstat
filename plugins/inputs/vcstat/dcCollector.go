// dcCollector gathers stats at Datacenter level
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vcstat

import (
	"context"
	"fmt"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

// dcCollector struct contains kid resources for a Datacenter
type dcCollector struct {
	dcs      []*object.Datacenter
	clusters map[int][]*object.ClusterComputeResource
	hosts    map[int][]*object.HostSystem
	nets     map[int][]object.NetworkReference
}

// NewDCCollector returns a new Collector exposing Datacenter stats.
func NewDCCollector(dcs []*object.Datacenter) (dcCollector, error) {
	res := dcCollector{
		dcs:      dcs,
		clusters: make(map[int][]*object.ClusterComputeResource),
		hosts:    make(map[int][]*object.HostSystem),
		nets:     make(map[int][]object.NetworkReference),
	}

	return res, nil
}

// Collect gathers datacenter info
func (c *dcCollector) Collect(
	ctx context.Context,
	client *vim25.Client,
	acc telegraf.Accumulator,
) error {
	var (
		err  error
		dcMo mo.Datacenter
	)

	finder := find.NewFinder(client, false)
	for i, dc := range c.dcs {
		finder.SetDatacenter(dc)

		// clusters
		c.clusters[i], err = finder.ClusterComputeResourceList(ctx, "*")
		if err != nil {
			return fmt.Errorf("could not get datacenter cluster list: %w", err)
		}

		// hosts
		c.hosts[i], err = finder.HostSystemList(ctx, "*")
		if err != nil {
			return fmt.Errorf("could not get datacenter node list: %w", err)
		}

		// networks (dvs,dvp,..)
		c.nets[i], err = finder.NetworkList(ctx, "*")
		if err != nil {
			return fmt.Errorf("could not get datacenter network list %w", err)
		}

		// Datacenter info (ref: https://github.com/vmware/govmomi/blob/master/govc/datacenter/info.go)
		err = dc.Properties(ctx, dc.Reference(), []string{"datastore", "network"}, &dcMo)
		if err != nil {
			return err
		}

		dctags := getDcTags(client.URL().Host, dc.Name(), dc.Reference().Value)
		dcfields := getDcFields(
			len(c.clusters[i]),
			len(c.hosts[i]),
			len(dcMo.Network),
			len(dcMo.Datastore),
		)
		acc.AddFields("vcstat_datacenter", dcfields, dctags, time.Now())
	}

	return nil
}

func getDcTags(vcenter, dcname, moid string) map[string]string {
	return map[string]string{
		"vcenter": vcenter,
		"dcname":  dcname,
		"moid":    moid,
	}
}

func getDcFields(clusters, hosts, datastores, networks int) map[string]interface{} {
	return map[string]interface{}{
		"num_clusters":   clusters,
		"num_datastores": datastores,
		"num_hosts":      hosts,
		"num_networks":   networks,
	}
}
