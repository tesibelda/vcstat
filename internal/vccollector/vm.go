// This file contains vccollector methods to gathers stats at vm level
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

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// CollectVmsInfo gathers basic virtual machine info
func (c *VcCollector) CollectVmsInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		vmtags                = make(map[string]string)
		vmfields              = make(map[string]interface{})
		vmMos                 []mo.VirtualMachine
		arefs                 []types.ManagedObjectReference
		host                  *object.HostSystem
		s                     *types.VirtualMachineSummary
		r                     *types.VirtualMachineRuntimeInfo
		k                     *types.VirtualMachineConfigSummary
		t                     time.Time
		hostname, clustername string
		err                   error
		exit                  bool
	)

	if c.client == nil || c.coll == nil {
		return fmt.Errorf("Could not get VMs info: %w", govplus.ErrorNoClient)
	}

	if err := c.getAllDatacentersVMs(ctx); err != nil {
		return fmt.Errorf("Could not get virtual machine entity list: %w", err)
	}

	for i, dc := range c.dcs {
		// get VM references and split the list into chunks
		for _, vm := range c.vms[i] {
			arefs = append(arefs, vm.Reference())
		}
		chunks := chunckMoRefSlice(arefs, c.queryBulkSize)

		for _, refs := range chunks {
			err = c.coll.Retrieve(ctx, refs, []string{"summary"}, &vmMos)
			if err != nil {
				if err, exit = govplus.IsHardQueryError(err); exit {
					return fmt.Errorf("Could not get vm list summary property: %w", err)
				}
				acc.AddError(
					fmt.Errorf("Could not get vm list summary property: %w", err),
				)
				continue
			}
			t = time.Now()

			for _, vm := range vmMos {
				s = &vm.Summary
				r = &s.Runtime
				k = &s.Config
				hostname = ""
				clustername = ""
				if host = c.getHostObjectFromReference(i, r.Host); host != nil {
					hostname = host.Name()
					clustername = c.getClusternameFromHost(i, host)
				}

				vmtags["clustername"] = clustername
				vmtags["dcname"] = dc.Name()
				vmtags["esxhostname"] = hostname
				vmtags["guesthostname"] = s.Guest.HostName
				vmtags["moid"] = vm.Self.Reference().Value
				vmtags["vcenter"] = c.client.Client.URL().Host
				vmtags["vmname"] = k.Name

				vmfields["connection_state"] = string(r.ConnectionState)
				vmfields["connection_state_code"] = vmConnectionStateCode(string(r.ConnectionState))
				vmfields["consolidation_needed"] = *(r.ConsolidationNeeded)
				vmfields["max_cpu_usage"] = r.MaxCpuUsage
				vmfields["max_mem_usage"] = int64(r.MaxMemoryUsage) * (1024 * 1024)
				vmfields["memory_size"] = int64(s.Config.MemorySizeMB) * (1024 * 1024)
				vmfields["num_eth_cards"] = k.NumEthernetCards
				vmfields["num_vdisks"] = k.NumVirtualDisks
				vmfields["num_vcpus"] = s.Config.NumCpu
				vmfields["power_state"] = string(r.PowerState)
				vmfields["power_state_code"] = vmPowerStateCode(string(r.PowerState))
				vmfields["status"] = string(s.OverallStatus)
				vmfields["status_code"] = entityStatusCode(s.OverallStatus)
				vmfields["template"] = k.Template

				acc.AddFields("vcstat_vm", vmfields, vmtags, t)
			}
		}
	}

	return nil
}

// vmPowerStateCode converts VM PowerStateCode to int16 for easy alerting
func vmPowerStateCode(state string) int16 {
	switch state {
	case "poweredOn":
		return 0
	case "suspended":
		return 1
	case "poweredOff":
		return 2
	default:
		return 3
	}
}

// vmConnectionStateCode converts VM ConnectionStateCode to int16 for easy alerting
func vmConnectionStateCode(state string) int16 {
	switch state {
	case "connected":
		return 0
	case "orphaned":
		return 1
	case "invalid":
		return 2
	case "disconnected":
		return 3
	case "inaccessible":
		return 4
	default:
		return 5
	}
}
