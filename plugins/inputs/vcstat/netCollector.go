// netCollector gathers stats at network level (like Distributed Virtual Switchs)
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vcstat

import (
	"context"
	"fmt"
	"strconv"
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

// CollectDVS gathers Distributed Virtual Switch info
func (c *netCollector) CollectDVS(
		ctx context.Context,
		client *vim25.Client,
		dcs []*object.Datacenter,
		netMap map[int][]object.NetworkReference,
		acc telegraf.Accumulator,
) error {
	var (
		nets []object.NetworkReference
		dvsMo mo.DistributedVirtualSwitch
		dvsConfig *(types.DVSConfigInfo)
		err error = nil
	)

	for i, dc := range dcs {
		nets = netMap[i]
		for _, net := range nets {
			switch net.Reference().Type {
			case "DistributedVirtualSwitch", "VmwareDistributedVirtualSwitch":
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

				dvstags := getDVSTags(
						client.URL().Host,
						dc.Name(),
						dvs.Name(),
						net.Reference().Value,
				)
				dvsfields := getDVSFields(
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

// Collect gathers Distributed Virtual Portgroup info
func (c *netCollector) CollectDVP(
		ctx context.Context,
		client *vim25.Client,
		dcs []*object.Datacenter,
		netMap map[int][]object.NetworkReference,
		acc telegraf.Accumulator,
) error {
	var (
		nets []object.NetworkReference
		dvpMo mo.DistributedVirtualPortgroup
		dvpConfig types.DVPortgroupConfigInfo
		err error = nil
	)

	for i, dc := range dcs {
		nets = netMap[i]
		for _, net := range nets {
			if net.Reference().Type == "DistributedVirtualPortgroup" {
				dvp, ok := net.(*object.DistributedVirtualPortgroup)
				if !ok {
					return fmt.Errorf("could not get DVP from networkreference")
				}
				err = dvp.Properties(
						ctx, dvp.Reference(),
						[]string{"config", "overallStatus"},
						&dvpMo,
				)
				if err != nil {
					*c = false
					return fmt.Errorf("could not get dvp config property: %w", err)
				}
				dvpConfig = dvpMo.Config

				dvptags := getDVPTags(
						client.URL().Host,
						dc.Name(),
						dvp.Name(),
						net.Reference().Value,
						strconv.FormatBool(*dvpConfig.Uplink),
				)
				dvpfields := getDVPFields(
						string(dvpMo.OverallStatus),
						entityStatusCode(dvpMo.OverallStatus),
						dvpConfig.NumPorts,
				)
				acc.AddFields("vcstat_net_dvp", dvpfields, dvptags, time.Now())
			}
		}
	}
	*c = true

	return nil
}

func getDVSTags(vcenter, dcname, dvs, moid string) map[string]string {
	return map[string]string{
		"vcenter": vcenter,
		"dcname":  dcname,
		"dvs":     dvs,
		"moid":    moid,
	}
}

func getDVSFields(
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

func getDVPTags(vcenter, dcname, dvp, moid, uplink string) map[string]string {
	return map[string]string{
		"vcenter": vcenter,
		"dcname":  dcname,
		"dvp":     dvp,
		"moid":    moid,
		"uplink":  uplink,
	}
}

func getDVPFields(
		overallstatus string,
		dvpstatuscode int16,
		numports int32,
) map[string]interface{} {
	return map[string]interface{}{
		"status":      overallstatus,
		"status_code": dvpstatuscode,
		"num_ports":   numports,
	}
}
