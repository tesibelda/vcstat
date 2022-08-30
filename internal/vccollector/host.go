// This file contains vccollector methods to gather stats about host entities
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vccollector

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/vmware/govmomi/govc/host/esxcli"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// CollectHostInfo gathers host info
func (c *VcCollector) CollectHostInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		hsMo                     mo.HostSystem
		hostSt                   *hostState
		err                      error
		hsCode, hsConnectionCode int16
	)

	if c.client == nil {
		return fmt.Errorf("Could not get host info: %w", Error_NoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("Could not find host state for %s", host.Name()))
				continue
			}
			err = host.Properties(ctx, host.Reference(), []string{"summary"}, &hsMo)
			if err != nil {
				if err, exit := govQueryError(err); exit {
					return fmt.Errorf(
						"Could not get host %s summary property: %w",
						host.Name(),
						err,
					)
				}
				acc.AddError(
					fmt.Errorf(
						"Could not get host %s summary property: %w",
						host.Name(),
						err,
					),
				)
				continue
			}
			s := hsMo.Summary
			r := s.Runtime
			h := s.Hardware
			hostSt.setNotConnected(
				r.ConnectionState != types.HostSystemConnectionStateConnected,
			)
			hsCode = entityStatusCode(s.OverallStatus)
			hsConnectionCode = hostConnectionStateCode(r.ConnectionState)

			hstags := getHostTags(
				c.client.Client.URL().Host,
				dc.Name(),
				c.getClusternameFromHost(i, host),
				host.Name(),
				host.Reference().Value,
			)
			hsfields := getHostFields(
				string(s.OverallStatus),
				hsCode,
				s.RebootRequired,
				r.InMaintenanceMode,
				string(r.ConnectionState),
				hsConnectionCode,
				h.MemorySize,
				h.NumCpuCores,
				h.CpuMhz,
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
		x         *esxcli.Executor
		res       *esxcli.Response
		hostSt    *hostState
		startTime time.Time
		err       error
	)

	if c.client == nil {
		return fmt.Errorf("Could not get host HBAs info: %w", Error_NoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("Could not find host state for %s", host.Name()))
				continue
			}
			if !hostSt.isHostConnectedAndResponding(c.skipNotRespondigFor) {
				continue
			}
			startTime = time.Now()
			if x, err = esxcli.NewExecutor(c.client.Client, host); err != nil {
				acc.AddError(
					fmt.Errorf(
						"Could not get esxcli executor for host %s: %w",
						host.Name(),
						err,
					),
				)
				continue
			}
			res, err = x.Run([]string{"storage", "core", "adapter", "list"})
			hostSt.setMeanResponseTime(time.Since(startTime))
			if err != nil {
				if err, exit := govQueryError(err); exit {
					return err
				}
				acc.AddError(
					fmt.Errorf(
						"Could not run esxcli storage executor against host %s: %w",
						host.Name(),
						err,
					),
				)
				hostSt.setNotResponding(true)
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
							c.getClusternameFromHost(i, host),
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
		x         *esxcli.Executor
		res       *esxcli.Response
		hostSt    *hostState
		startTime time.Time
		err       error
	)

	if c.client == nil {
		return fmt.Errorf("Could not get host NICs info: %w", Error_NoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("Could not find host state for %s", host.Name()))
				continue
			}
			if !hostSt.isHostConnectedAndResponding(c.skipNotRespondigFor) {
				continue
			}
			startTime = time.Now()
			if x, err = esxcli.NewExecutor(c.client.Client, host); err != nil {
				acc.AddError(fmt.Errorf("Could not find host state for %s", host.Name()))
				continue
			}
			res, err = x.Run([]string{"network", "nic", "list"})
			hostSt.setMeanResponseTime(time.Since(startTime))
			if err != nil {
				if err, exit := govQueryError(err); exit {
					return err
				}
				acc.AddError(
					fmt.Errorf(
						"Could not run esxcli network executor against host %s: %w",
						host.Name(),
						err,
					),
				)
				hostSt.setNotResponding(true)
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
							c.getClusternameFromHost(i, host),
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
		x         *esxcli.Executor
		res       *esxcli.Response
		hostSt    *hostState
		startTime time.Time
		err       error
	)

	if c.client == nil {
		return fmt.Errorf("Could not get host firewalls info: %w", Error_NoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("Could not find host state for %s", host.Name()))
				continue
			}
			if !hostSt.isHostConnectedAndResponding(c.skipNotRespondigFor) {
				continue
			}
			startTime = time.Now()
			if x, err = esxcli.NewExecutor(c.client.Client, host); err != nil {
				acc.AddError(
					fmt.Errorf(
						"Could not get esxcli executor for host %s: %w",
						host.Name(),
						err,
					),
				)
				continue
			}
			res, err = x.Run([]string{"network", "firewall", "get"})
			hostSt.setMeanResponseTime(time.Since(startTime))
			if err != nil {
				if err, exit := govQueryError(err); exit {
					return err
				}
				acc.AddError(
					fmt.Errorf(
						"Could not run esxcli firewall executor against host %s: %w",
						host.Name(),
						err,
					),
				)
				hostSt.setNotResponding(true)
				continue
			}

			if len(res.Values) > 0 && len(res.Values[0]["Enabled"]) > 0 {
				fwtags := getFirewallTags(
					c.client.Client.URL().Host,
					dc.Name(),
					c.getClusternameFromHost(i, host),
					host.Name(),
				)
				enabled, err := strconv.ParseBool(res.Values[0]["Enabled"][0])
				if err != nil {
					acc.AddError(
						fmt.Errorf(
							"Could not parse firewall info for host %s: %w",
							host.Name(),
							err,
						),
					)
					continue
				}
				loaded, err := strconv.ParseBool(res.Values[0]["Loaded"][0])
				if err != nil {
					acc.AddError(
						fmt.Errorf(
							"Could not parse firewall info for host %s: %w",
							host.Name(),
							err,
						),
					)
					continue
				}
				fwfields := getFirewallFields(
					res.Values[0]["DefaultAction"][0],
					enabled,
					loaded,
				)
				acc.AddFields("vcstat_host_firewall", fwfields, fwtags, time.Now())
			}
		}
	}

	return nil
}

// ReportHostEsxcliResponse reports metrics about host esxcli command responses
func (c *VcCollector) ReportHostEsxcliResponse(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		hostSt          *hostState
		responding_code int
	)

	if c.client == nil {
		return fmt.Errorf("Could not report host esxcli responses info: %w", Error_NoClient)
	}

	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("Could not find host state for %s", host.Name()))
				continue
			}

			hstags := getHostTags(
				c.client.Client.URL().Host,
				dc.Name(),
				c.getClusternameFromHost(i, host),
				host.Name(),
				host.Reference().Value,
			)
			responding_code = 0
			if !hostSt.isHostConnected() {
				responding_code = 1
			} else {
				if !hostSt.isHostConnectedAndResponding(c.skipNotRespondigFor) {
					responding_code = 2
				}
			}
			hsfields := getEsxcliFields(
				responding_code,
				int(hostSt.responseTime.Nanoseconds()),
			)

			acc.AddFields("vcstat_host_esxcli", hsfields, hstags, time.Now())
		}
	}

	return nil
}

func (c *VcCollector) getClusternameFromHost(dcindex int, host *object.HostSystem) string {
	for _, cluster := range c.clusters[dcindex] {
		if strings.HasPrefix(host.InventoryPath, cluster.InventoryPath+"/") {
			return cluster.Name()
		}
	}

	return ""
}

func (c *VcCollector) getHostObjectFromReference(
	dcindex int,
	r *types.ManagedObjectReference,
) *object.HostSystem {
	for _, host := range c.hosts[dcindex] {
		if host.Reference().Type == r.Type && host.Reference().Value == r.Value {
			return host
		}
	}

	return nil
}

// hostConnectionStateCode converts types.HostSystemConnectionState to int16 for easy
//  alerting from telegraf metrics
func hostConnectionStateCode(state types.HostSystemConnectionState) int16 {
	switch state {
	case types.HostSystemConnectionStateConnected:
		return 0
	case types.HostSystemConnectionStateNotResponding:
		return 1
	case types.HostSystemConnectionStateDisconnected:
		return 2
	default:
		return 0
	}
}

func getHostTags(vcenter, dcname, cluster, hostname, moid string) map[string]string {
	return map[string]string{
		"clustername": cluster,
		"dcname":      dcname,
		"esxhostname": hostname,
		"moid":        moid,
		"vcenter":     vcenter,
	}
}

func getHostFields(
	overallstatus string,
	hoststatuscode int16,
	rebootrequired, inmaintenancemode bool,
	connectionstate string,
	connectionstatecode int16,
	memorysize int64,
	numcpu int16,
	cpumhz int32,
) map[string]interface{} {
	return map[string]interface{}{
		"connection_state":      connectionstate,
		"connection_state_code": connectionstatecode,
		"in_maintenance_mode":   inmaintenancemode,
		"reboot_required":       rebootrequired,
		"status":                overallstatus,
		"status_code":           hoststatuscode,
		"memory_size":           memorysize,
		"num_cpus":              numcpu,
		"cpu_freq":              cpumhz,
	}
}

func getHbaTags(vcenter, dcname, cluster, hostname, hba, driver string) map[string]string {
	return map[string]string{
		"clustername": cluster,
		"dcname":      dcname,
		"device":      hba,
		"driver":      driver,
		"esxhostname": hostname,
		"vcenter":     vcenter,
	}
}

func getHbaFields(status string, statuscode int16) map[string]interface{} {
	return map[string]interface{}{
		"link_state":      status,
		"link_state_code": statuscode,
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

func getNicTags(vcenter, dcname, cluster, hostname, nic, driver string) map[string]string {
	return map[string]string{
		"clustername": cluster,
		"dcname":      dcname,
		"device":      nic,
		"driver":      driver,
		"esxhostname": hostname,
		"vcenter":     vcenter,
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

func getFirewallTags(vcenter, dcname, cluster, hostname string) map[string]string {
	return map[string]string{
		"clustername": cluster,
		"dcname":      dcname,
		"esxhostname": hostname,
		"vcenter":     vcenter,
	}
}

func getFirewallFields(defaultaction string, enabled, loaded bool) map[string]interface{} {
	return map[string]interface{}{
		"defaultaction": defaultaction,
		"enabled":       enabled,
		"loaded":        loaded,
	}
}

func getEsxcliFields(responding_code, response_time int) map[string]interface{} {
	return map[string]interface{}{
		"responding_code":  responding_code,
		"response_time_ns": response_time,
	}
}
