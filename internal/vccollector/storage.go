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
		dstags   map[string]string
		dsfields map[string]interface{}
		refs     []types.ManagedObjectReference
		dsMo     []mo.Datastore
		err      error
	)

	if c.client == nil {
		return fmt.Errorf("Could not get datastores info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersDatastores(ctx); err != nil {
		return fmt.Errorf("Could not get datastore entity list: %w", err)
	}

	// reserve map memory for tags and fields according to setDsTags and setDsFields
	dstags = make(map[string]string, 5)
	dsfields = make(map[string]interface{}, 5)

	pc := property.DefaultCollector(c.client.Client)

	for i, dc := range c.dcs {
		refs = nil
		for _, ds := range c.dss[i] {
			refs = append(refs, ds.Reference())
		}
		err = pc.Retrieve(ctx, refs, []string{"summary"}, &dsMo)
		if err != nil {
			if err, exit := govplus.IsHardQueryError(err); exit {
				return err
			}
			acc.AddError(fmt.Errorf("Could not retrieve summary for datastore: %w", err))
			continue
		}
		for _, ds := range dsMo {
			setDsTags(
				dstags,
				c.client.Client.URL().Host,
				dc.Name(),
				ds.Summary.Name,
				ds.Reference().Value,
				ds.Summary.Type,
			)
			setDsFields(
				dsfields,
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

func setDsTags(
	tags map[string]string,
	vcenter, dcname, dsname, moid, dstype string,
) {
	tags["dcname"] = dcname
	tags["dsname"] = dsname
	tags["moid"] = moid
	tags["type"] = dstype
	tags["vcenter"] = vcenter
}

func setDsFields(
	fields map[string]interface{},
	accessible bool, capacity, freespace, uncommitted int64, maintenance string,
) {
	fields["accessible"] = accessible
	fields["capacity"] = capacity
	fields["freespace"] = freespace
	fields["maintenance_mode"] = maintenance
	fields["uncommitted"] = uncommitted
}
