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
		hstags             = make(map[string]string)
		hsfields           = make(map[string]interface{})
		hsref              types.ManagedObjectReference
		hsMos              []mo.HostSystem
		arefs              []types.ManagedObjectReference
		host               *object.HostSystem
		hostSt             *hostState
		s                  *(types.HostListSummary)
		r                  *(types.HostRuntimeInfo)
		h                  *(types.HostHardwareSummary)
		t                  time.Time
		err                error
		hsCode, hsConnCode int16
	)

	if c.client == nil || c.coll == nil {
		return fmt.Errorf("could not get host info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		// get Host reference list and split it into chunks
		arefs = nil
		for j, host := range c.hosts[i] {
			if !c.filterHostMatch(i, host) {
				continue
			}
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("could not find host state idx entry for %s", host.Name()))
				continue
			}
			arefs = append(arefs, host.Reference())
		}
		chunks := chunckMoRefSlice(arefs, c.queryBulkSize)

		for _, refs := range chunks {
			err = c.coll.Retrieve(ctx, refs, []string{"name", "summary", "vm", "datastore"}, &hsMos)
			if err != nil {
				if exit, err := govplus.IsHardQueryError(err); exit {
					return err
				}
				acc.AddError(
					fmt.Errorf("could not retrieve summary for host reference list: %w", err),
				)
				continue
			}
			t = time.Now()

			for _, hsMo := range hsMos {
				s = &hsMo.Summary
				r = s.Runtime
				h = s.Hardware
				if hostSt = c.getHostState(i, hsMo.Name); hostSt == nil {
					acc.AddError(fmt.Errorf("could not find host state entry for %s", hsMo.Name))
					continue
				}

				hsref = hsMo.Self.Reference()
				host = c.getHostObjectFromReference(i, &hsref)
				hstags["clustername"] = c.getClusternameFromHost(i, host)
				hstags["dcname"] = dc.Name()
				hstags["esxhostname"] = hsMo.Name
				hstags["moid"] = hsMo.Self.Reference().Value
				hstags["vcenter"] = c.client.Client.URL().Host

				hostSt.setNotConnected(
					r.ConnectionState != types.HostSystemConnectionStateConnected,
				)
				hsCode = entityStatusCode(s.OverallStatus)
				hsConnCode = hostConnectionStateCode(r.ConnectionState)
				hsfields["connection_state"] = string(r.ConnectionState)
				hsfields["connection_state_code"] = hsConnCode
				hsfields["in_maintenance_mode"] = r.InMaintenanceMode
				hsfields["reboot_required"] = s.RebootRequired
				hsfields["status"] = string(s.OverallStatus)
				hsfields["status_code"] = hsCode
				hsfields["memory_size"] = h.MemorySize
				hsfields["num_cpus"] = h.NumCpuCores
				hsfields["cpu_freq"] = h.CpuMhz
				hsfields["num_vms"] = len(hsMo.Vm)
				hsfields["num_datastores"] = len(hsMo.Datastore)

				acc.AddFields("vcstat_host", hsfields, hstags, t)
			}
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
		hbatags      = make(map[string]string)
		hbafields    = make(map[string]interface{})
		x            *esxcli.Executor
		res          *esxcli.Response
		hostSt       *hostState
		startTime, t time.Time
		err          error
	)

	if c.client == nil {
		return fmt.Errorf("could not get host HBAs info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if !c.filterHostMatch(i, host) {
				continue
			}
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("could not find host state idx entry for %s", host.Name()))
				continue
			}
			if !hostSt.isHostConnectedAndResponding(c.skipNotRespondigFor) {
				continue
			}
			startTime = time.Now()
			if x, err = esxcli.NewExecutor(c.client.Client, host); err != nil {
				hostExecutorNewAddError(acc, host.Name(), err)
				continue
			}
			res, err = x.Run([]string{"storage", "core", "adapter", "list"})
			hostSt.setMeanResponseTime(time.Since(startTime))
			if err != nil {
				hostExecutorRunAddError(acc, "storage core", host.Name(), err)
				hostSt.setNotResponding(true)
				if exit, err := govplus.IsHardQueryError(err); exit {
					return err
				}
				continue
			}

			t = time.Now()
			for _, rv := range res.Values {
				if len(rv) > 0 && len(rv["LinkState"]) > 0 {
					hbatags["clustername"] = c.getClusternameFromHost(i, host)
					hbatags["dcname"] = dc.Name()
					hbatags["device"] = rv["HBAName"][0]
					hbatags["driver"] = rv["Driver"][0]
					hbatags["esxhostname"] = host.Name()
					hbatags["vcenter"] = c.client.Client.URL().Host

					hbafields["link_state"] = rv["LinkState"][0]
					hbafields["link_state_code"] = hbaLinkStateCode(rv["LinkState"][0])

					acc.AddFields("vcstat_host_hba", hbafields, hbatags, t)
				}
			}
			if t.Sub(startTime) >= c.maxResponseDuration {
				hostSt.setNotResponding(true)
				return fmt.Errorf("slow response from %s: %w", host.Name(), context.DeadlineExceeded)
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
		nictags      = make(map[string]string)
		nicfields    = make(map[string]interface{})
		x            *esxcli.Executor
		res          *esxcli.Response
		hostSt       *hostState
		startTime, t time.Time
		err          error
	)

	if c.client == nil {
		return fmt.Errorf("could not get host NICs info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if !c.filterHostMatch(i, host) {
				continue
			}
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("could not find host state idx entry for %s", host.Name()))
				continue
			}
			if !hostSt.isHostConnectedAndResponding(c.skipNotRespondigFor) {
				continue
			}
			startTime = time.Now()
			if x, err = esxcli.NewExecutor(c.client.Client, host); err != nil {
				hostExecutorNewAddError(acc, host.Name(), err)
				continue
			}
			res, err = x.Run([]string{"network", "nic", "list"})
			hostSt.setMeanResponseTime(time.Since(startTime))
			if err != nil {
				hostExecutorRunAddError(acc, "network nic", host.Name(), err)
				hostSt.setNotResponding(true)
				if exit, err := govplus.IsHardQueryError(err); exit {
					return err
				}
				continue
			}

			t = time.Now()
			for _, rv := range res.Values {
				if len(rv) > 0 && len(rv["LinkStatus"]) > 0 {
					nictags["clustername"] = c.getClusternameFromHost(i, host)
					nictags["dcname"] = dc.Name()
					nictags["device"] = rv["Name"][0]
					nictags["driver"] = rv["Driver"][0]
					nictags["esxhostname"] = host.Name()
					nictags["vcenter"] = c.client.Client.URL().Host

					nicfields["admin_status"] = rv["AdminStatus"][0]
					nicfields["link_status"] = rv["LinkStatus"][0]
					nicfields["link_status_code"] = nicLinkStatusCode(rv["LinkStatus"][0])
					nicfields["duplex"] = rv["Duplex"][0]
					nicfields["mac"] = rv["MACAddress"][0]
					nicfields["speed"] = rv["Speed"][0]

					acc.AddFields("vcstat_host_nic", nicfields, nictags, t)
				}
			}
			if t.Sub(startTime) >= c.maxResponseDuration {
				hostSt.setNotResponding(true)
				return fmt.Errorf("slow response from %s: %w", host.Name(), context.DeadlineExceeded)
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
		fwtags       = make(map[string]string)
		fwfields     = make(map[string]interface{})
		x            *esxcli.Executor
		res          *esxcli.Response
		hostSt       *hostState
		startTime, t time.Time
		err          error
	)

	if c.client == nil {
		return fmt.Errorf("could not get host firewalls info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if !c.filterHostMatch(i, host) {
				continue
			}
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("could not find host state idx entry for %s", host.Name()))
				continue
			}
			if !hostSt.isHostConnectedAndResponding(c.skipNotRespondigFor) {
				continue
			}
			startTime = time.Now()
			if x, err = esxcli.NewExecutor(c.client.Client, host); err != nil {
				hostExecutorNewAddError(acc, host.Name(), err)
				continue
			}
			res, err = x.Run([]string{"network", "firewall", "get"})
			hostSt.setMeanResponseTime(time.Since(startTime))
			if err != nil {
				hostExecutorRunAddError(acc, "network firewall", host.Name(), err)
				hostSt.setNotResponding(true)
				if exit, err := govplus.IsHardQueryError(err); exit {
					return err
				}
				continue
			}

			t = time.Now()
			if len(res.Values) > 0 && len(res.Values[0]["Enabled"]) > 0 {
				fwtags["clustername"] = c.getClusternameFromHost(i, host)
				fwtags["dcname"] = dc.Name()
				fwtags["esxhostname"] = host.Name()
				fwtags["vcenter"] = c.client.Client.URL().Host

				enabled, err := strconv.ParseBool(res.Values[0]["Enabled"][0])
				if err != nil {
					hostExecutorParseAddError(acc, "firewall", host.Name(), err)
					continue
				}
				loaded, err := strconv.ParseBool(res.Values[0]["Loaded"][0])
				if err != nil {
					hostExecutorParseAddError(acc, "firewall", host.Name(), err)
					continue
				}
				fwfields["defaultaction"] = res.Values[0]["DefaultAction"][0]
				fwfields["enabled"] = enabled
				fwfields["loaded"] = loaded

				acc.AddFields("vcstat_host_firewall", fwfields, fwtags, t)
			}
			if t.Sub(startTime) >= c.maxResponseDuration {
				hostSt.setNotResponding(true)
				return fmt.Errorf("slow response from %s: %w", host.Name(), context.DeadlineExceeded)
			}
		}
	}

	return nil
}

// CollectHostServices gathers host services info (like govc: host.service.ls)
func (c *VcCollector) CollectHostServices(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		hstags       = make(map[string]string)
		hsfields     = make(map[string]interface{})
		hsref, sref  types.ManagedObjectReference
		hsMos        []mo.HostServiceSystem
		hrefs, srefs []types.ManagedObjectReference
		host         *object.HostSystem
		s            *object.HostServiceSystem
		hostSt       *hostState
		t            time.Time
		err          error
	)

	if c.client == nil || c.coll == nil {
		return fmt.Errorf("could not get host services info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		// get HostServiceSystem references list and split it into chunks
		hrefs, srefs = nil, nil
		for j, host := range c.hosts[i] {
			if !c.filterHostMatch(i, host) {
				continue
			}
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("could not find host state idx entry for %s", host.Name()))
				continue
			}
			if s, err = host.ConfigManager().ServiceSystem(ctx); err != nil {
				return fmt.Errorf("could not get host service system: %w", err)
			}
			hrefs = append(hrefs, host.Reference())
			srefs = append(srefs, s.Reference())
		}
		chunks := chunckMoRefSlice(srefs, c.queryBulkSize)

		for _, refs := range chunks {
			err = c.coll.Retrieve(ctx, refs, []string{"serviceInfo.service"}, &hsMos)
			if err != nil {
				if exit, err := govplus.IsHardQueryError(err); exit {
					return err
				}
				acc.AddError(
					fmt.Errorf("could not retrieve info for host service reference list: %w", err),
				)
				continue
			}
			t = time.Now()

			for _, hsMo := range hsMos {
				services := hsMo.ServiceInfo.Service

				// find host of this service
				sref = hsMo.Self.Reference()
				hsref = findHostRefInServiceRefList(hrefs, srefs, hsMo.Self.Reference())
				if hsref.Type == "" {
					acc.AddError(
						fmt.Errorf("could not find host for service reference: %s", sref),
					)
					continue
				}
				host = c.getHostObjectFromReference(i, &hsref)
				hstags["clustername"] = c.getClusternameFromHost(i, host)
				hstags["dcname"] = dc.Name()
				hstags["esxhostname"] = host.Name()
				hstags["vcenter"] = c.client.Client.URL().Host

				for _, service := range services {
					hstags["key"] = service.Key
					hsfields["label"] = service.Label
					hsfields["policy"] = service.Policy
					hsfields["required"] = service.Required
					hsfields["running"] = service.Running
					acc.AddFields("vcstat_host_service", hsfields, hstags, t)
				}
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
		hstags         = make(map[string]string)
		hsfields       = make(map[string]interface{})
		hostSt         *hostState
		t              time.Time
		respondingCode int
	)

	t = time.Now()
	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if !c.filterHostMatch(i, host) {
				continue
			}
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("could not find host state idx entry for %s", host.Name()))
				continue
			}

			hstags["clustername"] = c.getClusternameFromHost(i, host)
			hstags["dcname"] = dc.Name()
			hstags["esxhostname"] = host.Name()
			hstags["moid"] = host.Reference().Value
			hstags["vcenter"] = c.client.Client.URL().Host

			respondingCode = 0
			if !hostSt.isHostConnected() {
				respondingCode = 1
			} else if !hostSt.isHostConnectedAndResponding(c.skipNotRespondigFor) {
				respondingCode = 2
			}
			hsfields["respondingCode"] = respondingCode
			hsfields["response_time_ns"] = int(hostSt.responseTime.Nanoseconds())

			acc.AddFields("vcstat_host_esxcli", hsfields, hstags, t)
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

func (c *VcCollector) filterHostMatch(i int, host *object.HostSystem) bool {
	if !c.filterHosts.Match(host.Name()) {
		return false
	}
	if !c.filterClusters.Match(c.getClusternameFromHost(i, host)) {
		return false
	}
	return true
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

func hostExecutorNewAddError(acc telegraf.Accumulator, host string, err error) {
	acc.AddError(
		fmt.Errorf(
			"could not get esxcli executor for host %s: %w",
			host,
			err,
		),
	)
}

func hostExecutorParseAddError(acc telegraf.Accumulator, executor, host string, err error) {
	acc.AddError(
		fmt.Errorf(
			"could not parse %s info for host %s: %w",
			executor,
			host,
			err,
		),
	)
}

func hostExecutorRunAddError(acc telegraf.Accumulator, executor, host string, err error) {
	acc.AddError(
		fmt.Errorf(
			"could not run esxcli %s executor against host %s: %w",
			executor,
			host,
			err,
		),
	)
}

func findHostRefInServiceRefList(
	hrs []types.ManagedObjectReference,
	srs []types.ManagedObjectReference,
	sr types.ManagedObjectReference,
) types.ManagedObjectReference {
	var hr types.ManagedObjectReference
	for k, r := range srs {
		if r == sr {
			hr = hrs[k]
			break
		}
	}
	return hr
}

// hostConnectionStateCode converts types.HostSystemConnectionState to int16 for easy alerting
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

// hbaLinkStateCode converts storage adapter Link State to int16
// for easy alerting
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
