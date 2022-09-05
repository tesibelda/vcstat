// This file contains vccollector methods to gather stats about network entities
//  (like Distributed Virtual Switches)
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vccollector

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/tesibelda/vcstat/pkg/govplus"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// CollectNetDVS gathers Distributed Virtual Switch info
func (c *VcCollector) CollectNetDVS(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		dvsMo     mo.DistributedVirtualSwitch
		dvstags   map[string]string
		dvsfields map[string]interface{}
		nets      []object.NetworkReference
		dvsConfig *(types.DVSConfigInfo)
		dvs       *object.DistributedVirtualSwitch
		err       error
		ok        bool
	)

	if c.client == nil {
		return fmt.Errorf("Could not get network DVSs info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersNetworks(ctx); err != nil {
		return fmt.Errorf("Could not get network entity list: %w", err)
	}

	// reserve map memory for tags and fields according to setDVSTags and setDVSFields
	dvstags = make(map[string]string, 4)
	dvsfields = make(map[string]interface{}, 5)

	for i, dc := range c.dcs {
		nets = c.nets[i]
		for _, net := range nets {
			switch net.Reference().Type {
			case "DistributedVirtualSwitch", "VmwareDistributedVirtualSwitch":
				if dvs, ok = net.(*object.DistributedVirtualSwitch); !ok {
					acc.AddError(fmt.Errorf("Could not get DVS from networkreference"))
					continue
				}
				err = dvs.Properties(
					ctx, dvs.Reference(),
					[]string{"config", "overallStatus"},
					&dvsMo,
				)
				if err != nil {
					if err, exit := govplus.IsHardQueryError(err); exit {
						return err
					}
					acc.AddError(fmt.Errorf("Could not get dvs config property: %w", err))
					continue
				}
				if dvsConfig = dvsMo.Config.GetDVSConfigInfo(); dvsConfig == nil {
					acc.AddError(fmt.Errorf("Could not get dvs configuration info"))
					continue
				}

				setDVSTags(
					dvstags,
					c.client.Client.URL().Host,
					dc.Name(),
					dvs.Name(),
					net.Reference().Value,
				)
				setDVSFields(
					dvsfields,
					string(dvsMo.OverallStatus),
					entityStatusCode(dvsMo.OverallStatus),
					dvsConfig.NumPorts,
					dvsConfig.MaxPorts,
					dvsConfig.NumStandalonePorts,
				)
				acc.AddFields("vcstat_net_dvs", dvsfields, dvstags, time.Now())
			}
		}
	}

	return nil
}

// CollectNetDVP gathers Distributed Virtual Portgroup info
func (c *VcCollector) CollectNetDVP(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		dvpMo     mo.DistributedVirtualPortgroup
		dvpConfig types.DVPortgroupConfigInfo
		dvptags   map[string]string
		dvpfields map[string]interface{}
		nets      []object.NetworkReference
		dvp       *object.DistributedVirtualPortgroup
		err       error
		ok        bool
	)

	if c.client == nil {
		return fmt.Errorf("Could not get network DVPs info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersNetworks(ctx); err != nil {
		return fmt.Errorf("Could not get network entity list: %w", err)
	}

	// reserve map memory for tags and fields according to setDVPTags and setDVPFields
	dvptags = make(map[string]string, 5)
	dvpfields = make(map[string]interface{}, 3)

	for i, dc := range c.dcs {
		nets = c.nets[i]
		for _, net := range nets {
			if net.Reference().Type == "DistributedVirtualPortgroup" {
				if dvp, ok = net.(*object.DistributedVirtualPortgroup); !ok {
					acc.AddError(fmt.Errorf("Could not get DVP from networkreference"))
					continue
				}
				err = dvp.Properties(
					ctx, dvp.Reference(),
					[]string{"config", "overallStatus"},
					&dvpMo,
				)
				if err != nil {
					if err, exit := govplus.IsHardQueryError(err); exit {
						return err
					}
					acc.AddError(fmt.Errorf("Could not get dvp config property: %w", err))
					continue
				}
				dvpConfig = dvpMo.Config

				setDVPTags(
					dvptags,
					c.client.Client.URL().Host,
					dc.Name(),
					dvp.Name(),
					net.Reference().Value,
					strconv.FormatBool(*dvpConfig.Uplink),
				)
				setDVPFields(
					dvpfields,
					string(dvpMo.OverallStatus),
					entityStatusCode(dvpMo.OverallStatus),
					dvpConfig.NumPorts,
				)
				acc.AddFields("vcstat_net_dvp", dvpfields, dvptags, time.Now())
			}
		}
	}

	return nil
}

func setDVSTags(
	tags map[string]string,
	vcenter, dcname, dvs, moid string,
) {
	tags["dcname"] = dcname
	tags["dvs"] = dvs
	tags["moid"] = moid
	tags["vcenter"] = vcenter
}

func setDVSFields(
	fields map[string]interface{},
	overallstatus string,
	dvsstatuscode int16,
	numports, maxports, numsaports int32,
) {
	fields["max_ports"] = maxports
	fields["num_ports"] = numports
	fields["num_standalone_ports"] = numsaports
	fields["status"] = overallstatus
	fields["status_code"] = dvsstatuscode
}

func setDVPTags(
	tags map[string]string,
	vcenter, dcname, dvp, moid, uplink string,
) {
	tags["dcname"] = dcname
	tags["dvp"] = dvp
	tags["moid"] = moid
	tags["uplink"] = uplink
	tags["vcenter"] = vcenter
}

func setDVPFields(
	fields map[string]interface{},
	overallstatus string,
	dvpstatuscode int16,
	numports int32,
) {
	fields["num_ports"] = numports
	fields["status"] = overallstatus
	fields["status_code"] = dvpstatuscode
}
