// netCollector gathers stats at network level (like Distributed Virtual Switchs)
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vcstat

import (
	"context"
	"fmt"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// netCollector type indicates succesfull DVS collection or not
type netCollector bool

// NewNetCollector returns a new Collector exposing DVS stats.
func NewNetCollector() (netCollector, error) {
	return netCollector(true), nil
}

// Collect gathers DVS info
func (c *netCollector) CollectDvs(
		ctx context.Context,
		client *vim25.Client,
		dcs []*object.Datacenter,
		netMap map[int][]object.NetworkReference,
		acc telegraf.Accumulator,
) error {
	var nets []object.NetworkReference
	var dvsMo mo.DistributedVirtualSwitch
	var dvsConfig *(types.DVSConfigInfo)
	var err error = nil
	//	var clusterStatusCode int16 = 0

	for i, dc := range dcs {
		nets = netMap[i]
		for _, net := range nets {
			if net.Reference().Type == "VmwareDistributedVirtualSwitch" {

				dvs, ok := net.(*object.DistributedVirtualSwitch)
				if !ok {
					return fmt.Errorf("could not get DVS from networkreference")
				}
				err = dvs.Properties(
						ctx, dvs.Reference(),
						[]string{"config", "overallStatus"},
						&dvsMo,
				)
				if err != nil {
					*c = false
					return fmt.Errorf("could not get dvs config property: %w", err)
				}
				dvsConfig = dvsMo.Config.GetDVSConfigInfo()
				if dvsConfig == nil {
					*c = false
					return fmt.Errorf("coud not get dvs configuration info")
				}

				dvstags := getDvsTags(
						client.URL().Host,
						dc.Name(),
						dvs.Name(),
						net.Reference().Value,
				)
				dvsfields := getDvsFields(
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
	*c = true

	return nil
}

func getDvsTags(vcenter, dcname, dvs, moid string) map[string]string {
	return map[string]string{
		"vcenter": vcenter,
		"dcname":  dcname,
		"dvs":     dvs,
		"moid":    moid,
	}
}

func getDvsFields(
		overallstatus string,
		dvsstatuscode int16,
		numports, maxports, numsaports int32,
) map[string]interface{} {
	return map[string]interface{}{
		"status":               overallstatus,
		"status_code":          dvsstatuscode,
		"num_ports":            numports,
		"max_ports":            maxports,
		"num_standalone_ports": numsaports,
	}
}
