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

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
)

// CollectVmsInfo gathers basic virtual machine info
func (c *VcCollector) CollectVmsInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		vmMo   mo.VirtualMachine
		err    error
		vmCode int16
		host   *object.HostSystem
		hostname, clustername string
	)

	if c.client == nil {
		return fmt.Errorf(string(Error_NoClient))
	}

	if err := c.getAllDatacentersVMs(ctx); err != nil {
		return fmt.Errorf("Could not get virtual machine entity list: %w", err)
	}

	for i, dc := range c.dcs {
		for _, vm := range c.vms[i] {
			err = vm.Properties(ctx, vm.Reference(), []string{"summary"}, &vmMo)
			if err != nil {
				if err, exit := govQueryError(err); exit {
					return fmt.Errorf(
						"Could not get vm %s summary property: %w",
						vm.Name(),
						err,
					)
				}
				acc.AddError(
					fmt.Errorf(
						"Could not get vm %s summary property: %w",
						vm.Name(),
						err,
					),
				)
				continue
			}
			s := vmMo.Summary
			r := s.Runtime
			t := s.Config
			vmCode = entityStatusCode(s.OverallStatus)
			hostname = ""
			clustername = ""
			if host = c.getHostObjectFromReference(i, r.Host); host != nil {
				hostname = host.Name()
				clustername = c.getClusternameFromHost(i, host)
			}

			vmtags := getVmTags(
				c.client.Client.URL().Host,
				dc.Name(),
				clustername,
				hostname,
				vm.Reference().Value,
				t.Name,
				s.Guest.HostName,
			)
			vmfields := getVmFields(
				string(s.OverallStatus),
				vmCode,
				string(r.PowerState),
				vmPowerStateCode(string(r.PowerState)),
				r.MemoryOverhead,
				int64(s.Config.MemorySizeMB)*(1024*1024),
				s.Config.NumCpu,
				t.NumEthernetCards,
				t.NumVirtualDisks,
				t.Template,
				*(r.ConsolidationNeeded),
			)
			acc.AddFields("vcstat_vm", vmfields, vmtags, time.Now())
		}
	}

	return nil
}

func getVmTags(
	vcenter, dcname, cluster, hostname, moid, vmname, guesthostname string,
) map[string]string {
	return map[string]string{
		"clustername":   cluster,
		"dcname":        dcname,
		"esxhostname":   hostname,
		"guesthostname": guesthostname,
		"moid":          moid,
		"vcenter":       vcenter,
		"vmname":        vmname,
	}
}

func getVmFields(
	overallstatus string,
	vmstatuscode int16,
	powerstate string,
	powerstatecode int16,
	memoryoverhead, memorysize int64,
	numcpu, numeth, numvdisk int32,
	template, consolidationneeded bool,
) map[string]interface{} {
	return map[string]interface{}{
		"consolidation_needed": consolidationneeded,
		"memory_size":      memorysize,
		"memory_overhead":  memoryoverhead,
		"num_eth_cards":    numeth,
		"num_vdisks":       numvdisk,
		"num_vcpus":        numcpu,
		"power_state":      powerstate,
		"power_state_code": powerstatecode,
		"status":           overallstatus,
		"status_code":      vmstatuscode,
		"template":         template,
	}
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
