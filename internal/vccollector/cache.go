// This file contains vccollector methods to cache vCenter entities
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vccollector

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
)

const (
	strAsterisk = "*"
)

var (
	findNotFoundError *find.NotFoundError
)

type hostState struct {
	notConnected   bool
	lastNoResponse time.Time
	notResponding  bool
	responseTime   time.Duration
}

type VcCache struct {
	lastDCUpdate time.Time                          //nolint
	lastCHUpdate time.Time                          //nolint
	lastDsUpdate time.Time                          //nolint
	lastNtUpdate time.Time                          //nolint
	lastVmUpdate time.Time                          //nolint
	dcs          []*object.Datacenter               //nolint
	clusters     [][]*object.ClusterComputeResource //nolint
	dss          [][]*object.Datastore              //nolint
	hosts        [][]*object.HostSystem             //nolint
	hostStates   [][]hostState                      //nolint
	nets         [][]object.NetworkReference        //nolint
	vms          [][]*object.VirtualMachine         //nolint
}

func (c *VcCollector) getDatacenters(ctx context.Context) error {
	var err error

	if time.Since(c.lastDCUpdate) < c.dataDuration {
		return nil
	}

	finder := find.NewFinder(c.client.Client, false)
	if c.dcs, err = finder.DatacenterList(ctx, strAsterisk); err != nil {
		return fmt.Errorf("could not get datacenter list: %w", err)
	}
	c.lastDCUpdate = time.Now()

	return err
}

func (c *VcCollector) getAllDatacentersClustersAndHosts(ctx context.Context) error {
	var (
		err              error
		numdcs, numhosts int
		platformChange   bool
	)

	if time.Since(c.lastCHUpdate) < c.dataDuration {
		return nil
	}
	err = c.getDatacenters(ctx)
	if err != nil {
		return err
	}

	numdcs = len(c.dcs)
	if numdcs != len(c.clusters) || numdcs != len(c.hosts) {
		if numdcs > 0 {
			c.clusters = make([][]*object.ClusterComputeResource, numdcs)
			c.hosts = make([][]*object.HostSystem, numdcs)
			c.hostStates = make([][]hostState, numdcs)
		} else {
			c.clusters = nil
			c.hosts = nil
			c.hostStates = nil
		}
		platformChange = true
	}

	for i, dc := range c.dcs {
		finder := find.NewFinder(c.client.Client, false)
		finder.SetDatacenter(dc)

		// clusters
		if c.clusters[i], err = finder.ClusterComputeResourceList(ctx, strAsterisk); err != nil {
			if !errors.As(err, &findNotFoundError) {
				return fmt.Errorf("could not get datacenter cluster list: %w", err)
			}
		}

		// hosts
		numhosts = len(c.hosts[i])
		if c.hosts[i], err = finder.HostSystemList(ctx, strAsterisk); err != nil {
			return fmt.Errorf("could not get datacenter node list: %w", err)
		}

		// keep hostStates between intervals except if the number of dcs or hosts changed
		platformChange = platformChange || numhosts != len(c.hosts[i])
		if platformChange {
			c.hostStates[i] = make([]hostState, len(c.hosts[i]))
		}
	}
	c.lastCHUpdate = time.Now()

	return nil
}

func (c *VcCollector) getAllDatacentersNetworks(ctx context.Context) error {
	if time.Since(c.lastNtUpdate) < c.dataDuration {
		return nil
	}
	err := c.getDatacenters(ctx)
	if err != nil {
		return err
	}

	numdcs := len(c.dcs)
	if numdcs != len(c.nets) {
		if numdcs > 0 {
			c.nets = make([][]object.NetworkReference, numdcs)
		} else {
			c.nets = nil
		}
	}

	for i, dc := range c.dcs {
		finder := find.NewFinder(c.client.Client, false)
		finder.SetDatacenter(dc)

		if c.nets[i], err = finder.NetworkList(ctx, strAsterisk); err != nil {
			if !errors.As(err, &findNotFoundError) {
				return fmt.Errorf("could not get datacenter network list: %w", err)
			}
		}
	}
	c.lastNtUpdate = time.Now()

	return nil
}

func (c *VcCollector) getAllDatacentersDatastores(ctx context.Context) error {
	if time.Since(c.lastDsUpdate) < c.dataDuration {
		return nil
	}
	err := c.getDatacenters(ctx)
	if err != nil {
		return err
	}

	numdcs := len(c.dcs)
	if numdcs != len(c.dss) {
		if numdcs > 0 {
			c.dss = make([][]*object.Datastore, numdcs)
		} else {
			c.dss = nil
		}
	}

	for i, dc := range c.dcs {
		finder := find.NewFinder(c.client.Client, false)
		finder.SetDatacenter(dc)

		if c.dss[i], err = finder.DatastoreList(ctx, strAsterisk); err != nil {
			if !errors.As(err, &findNotFoundError) {
				return fmt.Errorf("could not get datacenter datastore list: %w", err)
			}
		}
	}
	c.lastDsUpdate = time.Now()

	return nil
}

func (c *VcCollector) getAllDatacentersVMs(ctx context.Context) error {
	if time.Since(c.lastVmUpdate) < c.dataDuration {
		return nil
	}
	err := c.getAllDatacentersClustersAndHosts(ctx)
	if err != nil {
		return err
	}

	numdcs := len(c.dcs)
	if numdcs != len(c.vms) {
		if numdcs > 0 {
			c.vms = make([][]*object.VirtualMachine, numdcs)
		} else {
			c.vms = nil
		}
	}

	for i, dc := range c.dcs {
		finder := find.NewFinder(c.client.Client, false)
		finder.SetDatacenter(dc)

		if c.vms[i], err = finder.VirtualMachineList(ctx, strAsterisk); err != nil {
			if !errors.As(err, &findNotFoundError) {
				return fmt.Errorf("could not get virtual machine list: %w", err)
			}
		}
	}
	c.lastVmUpdate = time.Now()

	return nil
}

func (c *VcCollector) IsHostConnected(dc *object.Datacenter, host *object.HostSystem) bool {
	for i, searcdc := range c.dcs {
		if searcdc == dc {
			for j, searchost := range c.hosts[i] {
				if searchost == host {
					return !c.hostStates[i][j].notConnected
				}
			}
		}
	}

	return false
}

func (c *VcCollector) getHostState(dcindex int, name string) *hostState {
	if len(c.hostStates) <= dcindex || len(c.hosts) <= dcindex {
		return nil
	}
	for j, host := range c.hosts[dcindex] {
		if host.Name() == name {
			return &(c.hostStates[dcindex][j])
		}
	}
	return nil
}

func (c *VcCollector) getHostStateIdx(dcindex, hostindex int) *hostState {
	if len(c.hostStates) <= dcindex || len(c.hostStates[dcindex]) <= hostindex {
		return nil
	}
	if len(c.hosts) <= dcindex || len(c.hosts[dcindex]) <= hostindex {
		return nil
	}
	return &(c.hostStates[dcindex][hostindex])
}

// GetNumberNotRespondingHosts returns the number of hosts connected but not responding
// to esxcli commands
func (c *VcCollector) GetNumberNotRespondingHosts() int {
	var numnotresponding int

	for i := range c.dcs {
		for _, hState := range c.hostStates[i] {
			if hState.notResponding {
				numnotresponding++
			}
		}
	}

	return numnotresponding
}

// ResetResponseTimes set host states response times to 0
func (c *VcCollector) ResetResponseTimes() {
	for i := range c.dcs {
		for j := range c.hostStates[i] {
			c.hostStates[i][j].responseTime = 0
		}
	}
}

func (h *hostState) setNotConnected(conn bool) {
	h.notConnected = conn
}

func (h *hostState) setNotResponding(resp bool) {
	h.notResponding = resp
	if resp {
		h.lastNoResponse = time.Now()
	}
}

func (h *hostState) sumResponseTime(dur time.Duration) {
	h.responseTime += dur
}

func (h *hostState) isHostConnectedAndResponding(skipDuration time.Duration) bool {
	var connectedResponding bool

	if !h.notConnected {
		// limit notResponding in cache for skipDuration
		if !h.lastNoResponse.IsZero() && time.Since(h.lastNoResponse) > skipDuration {
			h.setNotResponding(false)
		}
		connectedResponding = !h.notResponding
	}

	return connectedResponding
}

func (h *hostState) isHostConnected() bool {
	return !h.notConnected
}
