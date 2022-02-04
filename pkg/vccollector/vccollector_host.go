// This file contains vccollector methods to gather stats about host entities
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

	"github.com/vmware/govmomi/govc/host/esxcli"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
)

// CollectHostInfo gathers host info
func (c *VcCollector) CollectHostInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		hosts                    []*object.HostSystem
		hsMo                     mo.HostSystem
		err                      error
		hsCode, hsConnectionCode int16
	)

	if c.client == nil {
		return fmt.Errorf(string(Error_NoClient))
	}
	if c.hosts == nil {
		if err = c.getAllDatacentersEntities(ctx); err != nil {
			return err
		}
	}

	for i, dc := range c.dcs {
		hosts = c.hosts[i]
		for _, host := range hosts {
			err = host.Properties(ctx, host.Reference(), []string{"summary"}, &hsMo)
			if err != nil {
				acc.AddError(fmt.Errorf("Could not get host summary property: %w", err))
				continue
			}
			hsCode = entityStatusCode(hsMo.Summary.OverallStatus)
			hsConnectionCode = hostConnectionStateCode(hsMo.Summary.Runtime.ConnectionState)

			hstags := getHostTags(
				c.client.Client.URL().Host,
				dc.Name(),
				host.Name(),
				host.Reference().Value,
			)
			hsfields := getHostFields(
				string(hsMo.Summary.OverallStatus),
				hsCode,
				hsMo.Summary.RebootRequired,
				hsMo.Summary.Runtime.InMaintenanceMode,
				string(hsMo.Summary.Runtime.ConnectionState),
				hsConnectionCode,
			)
			acc.AddFields("vcstat_host", hsfields, hstags, time.Now())
		}
	}

	return nil
}

// CollectHostHBA gathers host HBA info (like govc: storage core adapter list)
func (c *VcCollector) CollectHostHBA(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		hosts []*object.HostSystem
		x     *esxcli.Executor
		res   *esxcli.Response
		err   error
	)

	if c.client == nil {
		return fmt.Errorf(string(Error_NoClient))
	}
	if c.hosts == nil {
		if err = c.getAllDatacentersEntities(ctx); err != nil {
			return err
		}
	}

	for i, dc := range c.dcs {
		hosts = c.hosts[i]
		for _, host := range hosts {
			if x, err = esxcli.NewExecutor(c.client.Client, host); err != nil {
				acc.AddError(fmt.Errorf("Could not get esxcli executor for host %s: %w", host.Name(), err))
				continue
			}
			if res, err = x.Run([]string{"storage", "core", "adapter", "list"}); err != nil {
				acc.AddError(fmt.Errorf("Could not run esxcli storage executor against host %s: %w", host.Name(), err))
				continue
			}

			if len(res.Values) > 0 {
				var keys []string
				for key := range res.Values[0] {
					keys = append(keys, key) //nolint
				}
				for _, rv := range res.Values {
					if len(rv) > 0 && len(rv["LinkState"]) > 0 {
						hbatags := getHbaTags(
							c.client.Client.URL().Host,
							dc.Name(),
							host.Name(),
							rv["HBAName"][0],
							rv["Driver"][0],
						)
						hbafields := getHbaFields(
							rv["LinkState"][0],
							hbaLinkStateCode(rv["LinkState"][0]),
						)
						acc.AddFields("vcstat_host_hba", hbafields, hbatags, time.Now())
					}
				}
			}
		}
	}

	return nil
}

// CollectHostNIC gathers host NIC info (like govc: host.esxcli network nic list)
func (c *VcCollector) CollectHostNIC(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		hosts []*object.HostSystem
		x     *esxcli.Executor
		res   *esxcli.Response
		err   error
	)

	if c.client == nil {
		return fmt.Errorf(string(Error_NoClient))
	}
	if c.hosts == nil {
		if err = c.getAllDatacentersEntities(ctx); err != nil {
			return err
		}
	}

	for i, dc := range c.dcs {
		hosts = c.hosts[i]
		for _, host := range hosts {
			if x, err = esxcli.NewExecutor(c.client.Client, host); err != nil {
				acc.AddError(fmt.Errorf("Could not get esxcli executor for host %s: %w", host.Name(), err))
				continue
			}
			if res, err = x.Run([]string{"network", "nic", "list"}); err != nil {
				acc.AddError(fmt.Errorf("Could not run esxcli network executor against host %s: %w", host.Name(), err))
				continue
			}

			if len(res.Values) > 0 {
				var keys []string
				for key := range res.Values[0] {
					keys = append(keys, key) //nolint
				}
				for _, rv := range res.Values {
					if len(rv) > 0 && len(rv["LinkStatus"]) > 0 {
						nictags := getNicTags(
							c.client.Client.URL().Host,
							dc.Name(),
							host.Name(),
							rv["Name"][0],
							rv["Driver"][0],
						)
						nicfields := getNicFields(
							rv["LinkStatus"][0],
							nicLinkStatusCode(rv["LinkStatus"][0]),
							rv["AdminStatus"][0], rv["Duplex"][0],
							rv["Speed"][0], rv["MACAddress"][0],
						)
						acc.AddFields("vcstat_host_nic", nicfields, nictags, time.Now())
					}
				}
			}
		}
	}

	return nil
}

// CollectHostFw gathers host Firewall info (like govc: host.esxcli network firewall get)
func (c *VcCollector) CollectHostFw(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		hosts []*object.HostSystem
		x     *esxcli.Executor
		res   *esxcli.Response
		err   error
	)

	if c.client == nil {
		return fmt.Errorf(string(Error_NoClient))
	}
	if c.hosts == nil {
		if err = c.getAllDatacentersEntities(ctx); err != nil {
			return err
		}
	}

	for i, dc := range c.dcs {
		hosts = c.hosts[i]
		for _, host := range hosts {
			if x, err = esxcli.NewExecutor(c.client.Client, host); err != nil {
				acc.AddError(fmt.Errorf("Could not get esxcli executor for host %s: %w", host.Name(), err))
				continue
			}
			if res, err = x.Run([]string{"network", "firewall", "get"}); err != nil {
				acc.AddError(fmt.Errorf("Could not run esxcli firewall executor against host %s: %w", host.Name(), err))
				continue
			}

			if len(res.Values) > 0 {
				fwtags := getFirewallTags(c.client.Client.URL().Host, dc.Name(), host.Name())
				enabled, err := strconv.ParseBool(res.Values[0]["Enabled"][0])
				if err != nil {
					acc.AddError(fmt.Errorf("Could not parse firewall info for host %s: %w", host.Name(), err))
					continue
				}
				loaded, err := strconv.ParseBool(res.Values[0]["Loaded"][0])
				if err != nil {
					acc.AddError(fmt.Errorf("Could not parse firewall info for host %s: %w", host.Name(), err))
					continue
				}
				fwfields := getFirewallFields(res.Values[0]["DefaultAction"][0], enabled, loaded)
				acc.AddFields("vcstat_host_firewall", fwfields, fwtags, time.Now())
			}
		}
	}

	return nil
}

func getHostTags(vcenter, dcname, hostname, moid string) map[string]string {
	return map[string]string{
		"vcenter":     vcenter,
		"dcname":      dcname,
		"esxhostname": hostname,
		"moid":        moid,
	}
}

func getHostFields(
	overallstatus string,
	hoststatuscode int16,
	rebootrequired, inmaintenancemode bool,
	connectionstate string,
	connectionstatecode int16,
) map[string]interface{} {
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

func getNicFields(
	status string,
	statuscode int16,
	adminstatus, duplex, speed, mac string,
) map[string]interface{} {
	return map[string]interface{}{
		"admin_status":     adminstatus,
		"link_status":      status,
		"link_status_code": statuscode,
		"duplex":           duplex,
		"mac":              mac,
		"speed":            speed,
	}
}

// hbaLinkStateCode converts storage adapter Link State to int16
// for easy alerting from telegraf metrics
func hbaLinkStateCode(state string) int16 {
	switch state {
	case "link-up", "online":
		return 0
	case "link-n/a":
		return 1
	case "unbound":
		return 1
	case "link-down", "offline":
		return 3
	default:
		return 1
	}
}

// nicLinkStatusCode converts LinkStatus to int16 for easy alerting
// from telegraf metrics
func nicLinkStatusCode(state string) int16 {
	switch state {
	case "Up":
		return 0
	case "Unknown":
		return 1
	case "Down":
		return 2
	default:
		return 1
	}
}

func getFirewallTags(vcenter, dcname, hostname string) map[string]string {
	return map[string]string{
		"vcenter":     vcenter,
		"dcname":      dcname,
		"esxhostname": hostname,
	}
}

func getFirewallFields(defaultaction string, enabled, loaded bool) map[string]interface{} {
	return map[string]interface{}{
		"defaultaction": defaultaction,
		"enabled":       enabled,
		"loaded":        loaded,
	}
}
