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

	"github.com/tesibelda/vcstat/pkg/govplus"
)

// CollectDatacenterInfo gathers datacenter info
func (c *VcCollector) CollectDatacenterInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var err error

	if c.client == nil {
		return fmt.Errorf("Could not get datacenters info: %w", govplus.ErrorNoClient)
	}

	if err = c.getAllDatacentersEntities(ctx); err != nil {
		return fmt.Errorf("Could not get all datacenters entity lists: %w", err)
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
	err := c.getDatacenters(ctx)
	if err != nil {
		return err
	}

	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return err
	}
	if err = c.getAllDatacentersNetworks(ctx); err != nil {
		return err
	}
	if err = c.getAllDatacentersDatastores(ctx); err != nil {
		return err
	}

	return err
}

func getDcTags(vcenter, dcname, moid string) map[string]string {
	return map[string]string{
		"dcname":  dcname,
		"moid":    moid,
		"vcenter": vcenter,
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
