// This file contains vccollector methods to gathers stats about storage entities
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

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// CollectDatastoresInfo gathers info for all datastores in the datacenter
// (like govc datastore.info)
func (c *VcCollector) CollectDatastoresInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		dstags   = make(map[string]string)
		dsfields = make(map[string]interface{})
		dsMos    []mo.Datastore
		arefs    []types.ManagedObjectReference
		t        time.Time
		err      error
	)

	if c.client == nil || c.coll == nil {
		return fmt.Errorf("Could not get datastores info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersDatastores(ctx); err != nil {
		return fmt.Errorf("Could not get datastore entity list: %w", err)
	}

	for i, dc := range c.dcs {
		// get DS references and split the list into chunks
		for _, ds := range c.dss[i] {
			arefs = append(arefs, ds.Reference())
		}
		chunks := chunckMoRefSlice(arefs, c.queryBulkSize)

		for _, refs := range chunks {
			err = c.coll.Retrieve(ctx, refs, []string{"summary"}, &dsMos)
			if err != nil {
				if err, exit := govplus.IsHardQueryError(err); exit {
					return err
				}
				acc.AddError(
					fmt.Errorf("Could not retrieve summary for datastore reference list: %w", err),
				)
				continue
			}
			t = time.Now()

			for _, ds := range dsMos {
				dstags["dcname"] = dc.Name()
				dstags["dsname"] = ds.Summary.Name
				dstags["moid"] = ds.Self.Reference().Value
				dstags["type"] = ds.Summary.Type
				dstags["vcenter"] = c.client.Client.URL().Host

				dsfields["accessible"] = ds.Summary.Accessible
				dsfields["capacity"] = ds.Summary.Capacity
				dsfields["freespace"] = ds.Summary.FreeSpace
				dsfields["maintenance_mode"] = ds.Summary.Uncommitted
				dsfields["uncommitted"] = ds.Summary.MaintenanceMode

				acc.AddFields("vcstat_datastore", dsfields, dstags, t)
			}
		}
	}

	return nil
}
