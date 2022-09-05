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
	var (
		dctags   map[string]string
		dcfields map[string]interface{}
		err      error
	)

	if c.client == nil {
		return fmt.Errorf("Could not get datacenters info: %w", govplus.ErrorNoClient)
	}

	if err = c.getAllDatacentersEntities(ctx); err != nil {
		return fmt.Errorf("Could not get all datacenters entity lists: %w", err)
	}

	// reserve map memory for tags and fields according to setDcTags and setDcFields
	dctags = make(map[string]string, 3)
	dcfields = make(map[string]interface{}, 4)

	for i, dc := range c.dcs {
		setDcTags(
			dctags,
			c.client.Client.URL().Host,
			dc.Name(),
			dc.Reference().Value,
		)
		setDcFields(
			dcfields,
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

func setDcTags(
	tags map[string]string,
	vcenter, dcname, moid string,
) {
	tags["dcname"] = dcname
	tags["moid"] = moid
	tags["vcenter"] = vcenter
}

func setDcFields(
	fields map[string]interface{},
	clusters, hosts, datastores, networks int,
) {
	fields["num_clusters"] = clusters
	fields["num_datastores"] = datastores
	fields["num_hosts"] = hosts
	fields["num_networks"] = networks
}
