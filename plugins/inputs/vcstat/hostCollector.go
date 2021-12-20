// hsCollector gathers stats at host level
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vcstat

import (
	"context"
	"fmt"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/vmware/govmomi/govc/host/esxcli"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

// hsCollector type indicates succesfull host collection or not
type hsCollector bool

// NewHsCollector returns a new Collector exposing host stats.
func NewHsCollector() (hsCollector, error) {
	return hsCollector(true), nil
}

// Collect gathers host info
func (c *hsCollector) Collect(ctx context.Context, client *vim25.Client, dcs []*object.Datacenter, hsMap map[int][]*object.HostSystem, acc telegraf.Accumulator) error {
	var hosts []*object.HostSystem
	var hsMo mo.HostSystem
	var err error = nil
	var hsCode, hsConnectionCode int16 = 0, 0

	for i, dc := range dcs {
		hosts = hsMap[i]
		for _, host := range hosts {
			err = host.Properties(ctx, host.Reference(), []string{"summary"}, &hsMo)
			if err != nil {
				*c = false
				return fmt.Errorf("could not get host summary property: %w", err)
			}
			hsCode = entityStatusCode(hsMo.Summary.OverallStatus)
			hsConnectionCode = hostConnectionStateCode(hsMo.Summary.Runtime.ConnectionState)

			hstags := getHsTags(client.URL().Host, dc.Name(), host.Name(), host.Reference().Value)
			hsfields := getHsFields(string(hsMo.Summary.OverallStatus), hsCode, hsMo.Summary.RebootRequired, hsMo.Summary.Runtime.InMaintenanceMode, string(hsMo.Summary.Runtime.ConnectionState), hsConnectionCode)
			acc.AddFields("vcstat_host", hsfields, hstags, time.Now())
		}
	}
	*c = true

	return nil
}

// CollectHBA gathers host HBA info (like govc: storage core adapter list)
func (c *hsCollector) CollectHBA(ctx context.Context, client *vim25.Client, dcs []*object.Datacenter, hsMap map[int][]*object.HostSystem, acc telegraf.Accumulator) error {
	var hosts []*object.HostSystem

	for i, dc := range dcs {
		hosts = hsMap[i]
		for _, host := range hosts {
			x, err := esxcli.NewExecutor(client, host)
			if err != nil {
				return fmt.Errorf("could not get esxcli executor: %w", err)
			}
			res, err := x.Run([]string{"storage", "core", "adapter", "list"})
			if err != nil {
				return err
			}

			if len(res.Values) > 0 {
				var keys []string
				for key := range res.Values[0] {
					keys = append(keys, key)
				}
				for _, rv := range res.Values {
					hbatags := getHbaTags(client.URL().Host, dc.Name(), host.Name(), rv["HBAName"][0], rv["Driver"][0])
					hbafields := getHbaFields(rv["LinkState"][0], hbaLinkStateCode(rv["LinkState"][0]))
					acc.AddFields("vcstat_host_hba", hbafields, hbatags, time.Now())
				}
			} else {
				fmt.Println("no storage adapters found", host.Name(), res.String)
			}
		}
	}

	return nil
}

// CollectNIC gathers host NIC info (like govc: host.esxcli network nic list)
func (c *hsCollector) CollectNIC(ctx context.Context, client *vim25.Client, dcs []*object.Datacenter, hsMap map[int][]*object.HostSystem, acc telegraf.Accumulator) error {
	var hosts []*object.HostSystem

	for i, dc := range dcs {
		hosts = hsMap[i]
		for _, host := range hosts {
			x, err := esxcli.NewExecutor(client, host)
			if err != nil {
				return fmt.Errorf("could not get esxcli executor: %w", err)
			}
			res, err := x.Run([]string{"network", "nic", "list"})
			if err != nil {
				return err
			}

			if len(res.Values) > 0 {
				var keys []string
				for key := range res.Values[0] {
					keys = append(keys, key)
				}
				for _, rv := range res.Values {
					nictags := getNicTags(client.URL().Host, dc.Name(), host.Name(), rv["Name"][0], rv["Driver"][0])
					nicfields := getNicFields(rv["LinkStatus"][0], nicLinkStatusCode(rv["LinkStatus"][0]), rv["AdminStatus"][0], rv["Duplex"][0], rv["Speed"][0], rv["MACAddress"][0])
					acc.AddFields("vcstat_host_nic", nicfields, nictags, time.Now())
				}
			}
		}
	}

	return nil
}

func getHsTags(vcenter, dcname, hostname, moid string) map[string]string {
	return map[string]string{
		"vcenter":     vcenter,
		"dcname":      dcname,
		"esxhostname": hostname,
		"moid":        moid,
	}
}

func getHsFields(overallstatus string, hoststatuscode int16, rebootrequired, inmaintenancemode bool, connectionstate string, connectionstatecode int16) map[string]interface{} {
	return map[string]interface{}{
		"status":                overallstatus,
		"status_code":           hoststatuscode,
		"reboot_required":       rebootrequired,
		"in_maintenance_mode":   inmaintenancemode,
		"connection_state":      connectionstate,
		"connection_state_code": connectionstatecode,
	}
}

func getHbaTags(vcenter, dcname, hostname, hba, driver string) map[string]string {
	return map[string]string{
		"vcenter":     vcenter,
		"dcname":      dcname,
		"esxhostname": hostname,
		"device":      hba,
		"driver":      driver,
	}
}

func getHbaFields(status string, statuscode int16) map[string]interface{} {
	return map[string]interface{}{
		"link_state":      status,
		"link_state_code": statuscode,
	}
}

func getNicTags(vcenter, dcname, hostname, nic, driver string) map[string]string {
	return map[string]string{
		"vcenter":     vcenter,
		"dcname":      dcname,
		"esxhostname": hostname,
		"device":      nic,
		"driver":      driver,
	}
}

func getNicFields(status string, statuscode int16, adminStatus, duplex, speed, max string) map[string]interface{} {
	return map[string]interface{}{
		"link_status":      status,
		"link_status_code": statuscode,
	}
}
