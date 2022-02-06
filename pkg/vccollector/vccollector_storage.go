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

	"github.com/vmware/govmomi/property"
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
		refs []types.ManagedObjectReference
		dsMo []mo.Datastore
		err  error
	)

	if c.client == nil {
		return fmt.Errorf(string(Error_NoClient))
	}
	if c.dss == nil {
		if err = c.getAllDatacentersEntities(ctx); err != nil {
			return err
		}
	}

	pc := property.DefaultCollector(c.client.Client)

	for i, dc := range c.dcs {
		refs = nil
		for _, ds := range c.dss[i] {
			refs = append(refs, ds.Reference())
		}
		err = pc.Retrieve(ctx, refs, []string{"summary"}, &dsMo)
		if err != nil {
			if err, exit := govQueryError(err); exit {
				return err
			}
			acc.AddError(fmt.Errorf("Could not retrieve summary for datastore: %w", err))
			continue
		}
		for _, ds := range dsMo {
			dstags := getDsTags(
				c.client.Client.URL().Host,
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
