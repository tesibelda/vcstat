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

	"github.com/tesibelda/vcstat/pkg/govplus"

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
		hstags                   map[string]string
		hsfields                 map[string]interface{}
		hsMo                     mo.HostSystem
		err                      error
		hostSt                   *hostState
		s                        *(types.HostListSummary)
		r                        *(types.HostRuntimeInfo)
		h                        *(types.HostHardwareSummary)
		hsCode, hsConnectionCode int16
		exit                     bool
	)

	if c.client == nil {
		return fmt.Errorf("Could not get host info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

	// reserve map memory for tags and fields according to setHostTags and setHostFields
	hstags = make(map[string]string, 5)
	hsfields = make(map[string]interface{}, 9)

	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("Could not find host state for %s", host.Name()))
				continue
			}
			err = host.Properties(ctx, host.Reference(), []string{"summary"}, &hsMo)
			if err != nil {
				if err, exit = govplus.IsHardQueryError(err); exit {
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
			s = &hsMo.Summary
			r = s.Runtime
			h = s.Hardware
			hostSt.setNotConnected(
				r.ConnectionState != types.HostSystemConnectionStateConnected,
			)
			hsCode = entityStatusCode(s.OverallStatus)
			hsConnectionCode = hostConnectionStateCode(r.ConnectionState)

			setHostTags(
				hstags,
				c.client.Client.URL().Host,
				dc.Name(),
				c.getClusternameFromHost(i, host),
				host.Name(),
				host.Reference().Value,
			)
			setHostFields(
				hsfields,
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
		hbatags   map[string]string
		hbafields map[string]interface{}
		startTime time.Time
		err       error
		x         *esxcli.Executor
		res       *esxcli.Response
		hostSt    *hostState
	)

	if c.client == nil {
		return fmt.Errorf("Could not get host HBAs info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

	// reserve map memory for tags and fields according to setHbaTags and setHbaFields
	hbatags = make(map[string]string, 6)
	hbafields = make(map[string]interface{}, 2)

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
				if err, exit := govplus.IsHardQueryError(err); exit {
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
						setHbaTags(
							hbatags,
							c.client.Client.URL().Host,
							dc.Name(),
							c.getClusternameFromHost(i, host),
							host.Name(),
							rv["HBAName"][0],
							rv["Driver"][0],
						)
						setHbaFields(
							hbafields,
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
		nictags   map[string]string
		nicfields map[string]interface{}
		startTime time.Time
		err       error
		x         *esxcli.Executor
		res       *esxcli.Response
		hostSt    *hostState
	)

	if c.client == nil {
		return fmt.Errorf("Could not get host NICs info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

	// reserve map memory for tags and fields according to setNicTags and setNicFields
	nictags = make(map[string]string, 6)
	nicfields = make(map[string]interface{}, 6)

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
				if err, exit := govplus.IsHardQueryError(err); exit {
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
						setNicTags(
							nictags,
							c.client.Client.URL().Host,
							dc.Name(),
							c.getClusternameFromHost(i, host),
							host.Name(),
							rv["Name"][0],
							rv["Driver"][0],
						)
						setNicFields(
							nicfields,
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
		fwtags    map[string]string
		fwfields  map[string]interface{}
		startTime time.Time
		err       error
		x         *esxcli.Executor
		res       *esxcli.Response
		hostSt    *hostState
	)

	if c.client == nil {
		return fmt.Errorf("Could not get host firewalls info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

	// reserve map memory for tags and fields according to setFirewallTags and setFirewallFields
	fwtags = make(map[string]string, 3)
	fwfields = make(map[string]interface{}, 2)

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
				if err, exit := govplus.IsHardQueryError(err); exit {
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
				setFirewallTags(
					fwtags,
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
				setFirewallFields(
					fwfields,
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
		hstags          map[string]string
		hsfields        map[string]interface{}
		hostSt          *hostState
		responding_code int
	)

	if c.client == nil {
		return fmt.Errorf("Could not report host esxcli responses info: %w", govplus.ErrorNoClient)
	}

	// reserve map memory for tags and fields according to setHostTags and setEsxcliFields
	hstags = make(map[string]string, 5)
	hsfields = make(map[string]interface{}, 2)

	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("Could not find host state for %s", host.Name()))
				continue
			}

			setHostTags(
				hstags,
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
			setEsxcliFields(
				hsfields,
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

func setHostTags(
	tags map[string]string,
	vcenter, dcname, cluster, hostname, moid string,
) {
	tags["clustername"] = cluster
	tags["dcname"] = dcname
	tags["esxhostname"] = hostname
	tags["moid"] = moid
	tags["vcenter"] = vcenter
}

func setHostFields(
	fields map[string]interface{},
	overallstatus string,
	hoststatuscode int16,
	rebootrequired, inmaintenancemode bool,
	connectionstate string,
	connectionstatecode int16,
	memorysize int64,
	numcpu int16,
	cpumhz int32,
) {
	fields["connection_state"] = connectionstate
	fields["connection_state_code"] = connectionstatecode
	fields["in_maintenance_mode"] = inmaintenancemode
	fields["reboot_required"] = rebootrequired
	fields["status"] = overallstatus
	fields["status_code"] = hoststatuscode
	fields["memory_size"] = memorysize
	fields["num_cpus"] = numcpu
	fields["cpu_freq"] = cpumhz
}

func setHbaTags(
	tags map[string]string,
	vcenter, dcname, cluster, hostname, hba, driver string,
) {
	tags["clustername"] = cluster
	tags["dcname"] = dcname
	tags["device"] = hba
	tags["driver"] = driver
	tags["esxhostname"] = hostname
	tags["vcenter"] = vcenter
}

func setHbaFields(
	fields map[string]interface{},
	status string, statuscode int16,
) {
	fields["link_state"] = status
	fields["link_state_code"] = statuscode
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

func setNicTags(
	tags map[string]string,
	vcenter, dcname, cluster, hostname, nic, driver string,
) {
	tags["clustername"] = cluster
	tags["dcname"] = dcname
	tags["device"] = nic
	tags["driver"] = driver
	tags["esxhostname"] = hostname
	tags["vcenter"] = vcenter
}

func setNicFields(
	fields map[string]interface{},
	status string,
	statuscode int16,
	adminstatus, duplex, speed, mac string,
) {
	fields["admin_status"] = adminstatus
	fields["link_status"] = status
	fields["link_status_code"] = statuscode
	fields["duplex"] = duplex
	fields["mac"] = mac
	fields["speed"] = speed
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

func setFirewallTags(
	tags map[string]string,
	vcenter, dcname, cluster, hostname string,
) {
	tags["clustername"] = cluster
	tags["dcname"] = dcname
	tags["esxhostname"] = hostname
	tags["vcenter"] = vcenter
}

func setFirewallFields(
	fields map[string]interface{},
	defaultaction string, enabled, loaded bool,
) {
	fields["defaultaction"] = defaultaction
	fields["enabled"] = enabled
	fields["loaded"] = loaded
}

func setEsxcliFields(
	fields map[string]interface{},
	responding_code, response_time int,
) {
	fields["responding_code"] = responding_code
	fields["response_time_ns"] = response_time
}
