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
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// dcCollector struct contains kid resources for a Datacenter
type dcCollector struct {
	dcs      []*object.Datacenter
	clusters map[int][]*object.ClusterComputeResource
	dss      map[int][]*object.Datastore
	hosts    map[int][]*object.HostSystem
	nets     map[int][]object.NetworkReference
}

// NewDCCollector returns a new Collector exposing Datacenter stats.
func NewDCCollector(dcs []*object.Datacenter) (dcCollector, error) {
	res := dcCollector{
		dcs:      dcs,
		clusters: make(map[int][]*object.ClusterComputeResource),
		dss:      make(map[int][]*object.Datastore),
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
	var err error

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

		// datastores
		c.dss[i], err = finder.DatastoreList(ctx, "*")
		if err != nil {
			return fmt.Errorf("could not get datacenter datastore list %w", err)
		}

		dctags := getDcTags(client.URL().Host, dc.Name(), dc.Reference().Value)
		dcfields := getDcFields(
			len(c.clusters[i]),
			len(c.dss[i]),
			len(c.hosts[i]),
			len(c.nets[i]),
		)
		acc.AddFields("vcstat_datacenter", dcfields, dctags, time.Now())
	}

	return nil
}

// CollectDatastoresInfo gathers info for all datastores in the datacenter
// (like govc datastore.info)
func (c *dcCollector) CollectDatastoresInfo(
	ctx context.Context,
	client *vim25.Client,
	acc telegraf.Accumulator,
) error {
	var (
		err  error
		refs []types.ManagedObjectReference
		dsMo []mo.Datastore
	)

	pc := property.DefaultCollector(client)

	for i, dc := range c.dcs {
		refs = nil
		for _, ds := range c.dss[i] {
			refs = append(refs, ds.Reference())
		}
		err = pc.Retrieve(ctx, refs, []string{"summary"}, &dsMo)
		if err != nil {
			return err
		}
		for _, ds := range dsMo {
			dstags := getDsTags(
				client.URL().Host,
				dc.Name(),
				ds.Summary.Name,
				ds.Reference().Value,
				ds.Summary.Type,
			)
			dsfields := getDsFields(
				ds.Summary.Accessible,
				ds.Summary.Capacity,
				ds.Summary.FreeSpace,
				ds.Summary.Uncommitted,
				ds.Summary.MaintenanceMode,
			)
			acc.AddFields("vcstat_datastore", dsfields, dstags, time.Now())
		}
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

func getDsTags(vcenter, dcname, dsname, moid, dstype string) map[string]string {
	return map[string]string{
		"vcenter": vcenter,
		"dcname":  dcname,
		"dsname":  dsname,
		"moid":    moid,
		"type":    dstype,
	}
}

func getDsFields(accessible bool, capacity, freespace, uncommited int64, maintenance string) map[string]interface{} {
	return map[string]interface{}{
		"accessible":       accessible,
		"capacity":         capacity,
		"freespace":        freespace,
		"uncommited":       uncommited,
		"maintenance_mode": maintenance,
	}
}
