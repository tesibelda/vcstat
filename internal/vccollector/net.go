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

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// CollectNetDVS gathers Distributed Virtual Switch info
func (c *VcCollector) CollectNetDVS(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		dvstags   = make(map[string]string)
		dvsfields = make(map[string]interface{})
		arefs     []types.ManagedObjectReference
		dvsMos    []mo.DistributedVirtualSwitch
		dvsConfig *(types.DVSConfigInfo)
		t         time.Time
		err       error
		exit      bool
	)

	if c.client == nil || c.coll == nil {
		return fmt.Errorf("could not get network DVSs info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersNetworks(ctx); err != nil {
		return fmt.Errorf("could not get network entity list: %w", err)
	}

	for i, dc := range c.dcs {
		// get Network references and split the list into chunks
		for _, net := range c.nets[i] {
			switch net.Reference().Type {
			case "DistributedVirtualSwitch", "VmwareDistributedVirtualSwitch":
				arefs = append(arefs, net.Reference())
			}
		}
		chunks := chunckMoRefSlice(arefs, c.queryBulkSize)

		for _, refs := range chunks {
			err = c.coll.Retrieve(ctx, refs, []string{"config", "overallStatus"}, &dvsMos)
			if err != nil {
				if err, exit = govplus.IsHardQueryError(err); exit {
					return err
				}
				acc.AddError(
					fmt.Errorf("could not get config property for DVS reference list: %w", err),
				)
				continue
			}
			t = time.Now()

			for _, dvs := range dvsMos {
				if dvsConfig = dvs.Config.GetDVSConfigInfo(); dvsConfig == nil {
					acc.AddError(fmt.Errorf("could not get DVS configuration info"))
					continue
				}

				dvstags["dcname"] = dc.Name()
				dvstags["dvs"] = dvsConfig.Name
				dvstags["moid"] = dvs.Self.Value
				dvstags["vcenter"] = c.client.Client.URL().Host

				dvsfields["max_ports"] = dvsConfig.MaxPorts
				dvsfields["num_hosts"] = len(dvsConfig.Host)
				dvsfields["num_ports"] = dvsConfig.NumPorts
				dvsfields["num_standalone_ports"] = dvsConfig.NumStandalonePorts
				dvsfields["pnic_capacity_ratio_for_reservation"] = dvsConfig.PnicCapacityRatioForReservation
				dvsfields["status"] = string(dvs.OverallStatus)
				dvsfields["status_code"] = entityStatusCode(dvs.OverallStatus)

				acc.AddFields("vcstat_net_dvs", dvsfields, dvstags, t)
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
		dvpConfig types.DVPortgroupConfigInfo
		dvptags   = make(map[string]string)
		dvpfields = make(map[string]interface{})
		arefs     []types.ManagedObjectReference
		dvpMos    []mo.DistributedVirtualPortgroup
		t         time.Time
		err       error
		exit      bool
	)

	if c.client == nil || c.coll == nil {
		return fmt.Errorf("could not get network DVPs info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersNetworks(ctx); err != nil {
		return fmt.Errorf("could not get network entity list: %w", err)
	}

	for i, dc := range c.dcs {
		// get Network references and split the list into chunks
		for _, net := range c.nets[i] {
			if net.Reference().Type == "DistributedVirtualPortgroup" {
				arefs = append(arefs, net.Reference())
			}
		}
		chunks := chunckMoRefSlice(arefs, c.queryBulkSize)

		for _, refs := range chunks {
			err = c.coll.Retrieve(ctx, refs, []string{"config", "overallStatus"}, &dvpMos)
			if err != nil {
				if err, exit = govplus.IsHardQueryError(err); exit {
					return fmt.Errorf("could not get DVP list config property: %w", err)
				}
				acc.AddError(
					fmt.Errorf("could not get DVP list config property: %w", err),
				)
				continue
			}
			t = time.Now()

			for _, dvp := range dvpMos {
				dvpConfig = dvp.Config

				dvptags["dcname"] = dc.Name()
				dvptags["dvp"] = dvpConfig.Name
				dvptags["moid"] = dvp.Self.Value
				dvptags["uplink"] = strconv.FormatBool(*dvpConfig.Uplink)
				dvptags["vcenter"] = c.client.Client.URL().Host

				dvpfields["num_ports"] = dvpConfig.NumPorts
				dvpfields["status"] = string(dvp.OverallStatus)
				dvpfields["status_code"] = entityStatusCode(dvp.OverallStatus)

				acc.AddFields("vcstat_net_dvp", dvpfields, dvptags, t)
			}
		}
	}

	return nil
}
