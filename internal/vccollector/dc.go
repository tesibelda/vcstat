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
		dctags   = make(map[string]string)
		dcfields = make(map[string]interface{})
		t        time.Time
		err      error
	)

	if c.client == nil {
		return fmt.Errorf("could not get datacenters info: %w", govplus.ErrorNoClient)
	}

	if err = c.getAllDatacentersEntities(ctx); err != nil {
		return fmt.Errorf("could not get all datacenters entity lists: %w", err)
	}
	t = time.Now()

	for i, dc := range c.dcs {
		dctags["dcname"] = dc.Name()
		dctags["moid"] = dc.Reference().Value
		dctags["vcenter"] = c.client.Client.URL().Host

		dcfields["num_clusters"] = len(c.clusters[i])
		dcfields["num_datastores"] = len(c.dss[i])
		dcfields["num_hosts"] = len(c.hosts[i])
		dcfields["num_networks"] = len(c.nets[i])

		acc.AddFields("vcstat_datacenter", dcfields, dctags, t)
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
