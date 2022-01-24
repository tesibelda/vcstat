// This file contains vccollector methods to gathers stats about datacenters
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vccollector

import (
	"context"
	"fmt"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
)

// CollectDatacenterInfo gathers datacenter info
func (c *VcCollector) CollectDatacenterInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var err error

	if c.client == nil {
		fmt.Errorf(Error_NoClient)
	}

	if err = c.getAllDatacentersEntities(ctx); err != nil {
		return err
	}

	for i, dc := range c.dcs {
		dctags := getDcTags(
			c.client.Client.URL().Host,
			dc.Name(),
			dc.Reference().Value,
		)
		dcfields := getDcFields(
			len(c.clusters[i]),
			len(c.hosts[i]),
			len(c.dss[i]),
			len(c.nets[i]),
		)
		acc.AddFields("vcstat_datacenter", dcfields, dctags, time.Now())
	}

	return err
}

func (c *VcCollector) getAllDatacentersEntities(ctx context.Context) error {
	var (
		numdcs, numdcsbefore int
		err                  error
	)

	if c.dcs == nil {
		if err := c.getDatacenters(ctx); err != nil {
			return err
		}
	}

	// resize VcCollector number of DCs changed
	numdcs = len(c.dcs)
	numdcsbefore = len(c.clusters)
	if numdcs != numdcsbefore {
		if numdcs > 0 {
			c.clusters = make([][]*object.ClusterComputeResource, numdcs)
			c.dss = make([][]*object.Datastore, numdcs)
			c.hosts = make([][]*object.HostSystem, numdcs)
			c.nets = make([][]object.NetworkReference, numdcs)
		} else {
			c.clusters = nil
			c.dss = nil
			c.hosts = nil
			c.nets = nil
		}
	}

	for i, dc := range c.dcs {
		if err = c.getDatacenterEntities(ctx, dc, i); err != nil {
			return err
		}
	}

	return err
}

func (c *VcCollector) getDatacenterEntities(
	ctx context.Context,
	dc *object.Datacenter,
	idx int,
) error {
	var err error

	finder := find.NewFinder(c.client.Client, false)
	finder.SetDatacenter(dc)

	// clusters
	if c.clusters[idx], err = finder.ClusterComputeResourceList(ctx, "*"); err != nil {
		return fmt.Errorf("Could not get datacenter cluster list: %w", err)
	}

	// hosts
	if c.hosts[idx], err = finder.HostSystemList(ctx, "*"); err != nil {
		return fmt.Errorf("Could not get datacenter node list: %w", err)
	}

	// networks (dvs,dvp,..)
	if c.nets[idx], err = finder.NetworkList(ctx, "*"); err != nil {
		return fmt.Errorf("Could not get datacenter network list %w", err)
	}

	// datastores
	if c.dss[idx], err = finder.DatastoreList(ctx, "*"); err != nil {
		return fmt.Errorf("Could not get datacenter datastore list %w", err)
	}

	return err
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