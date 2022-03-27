// This file contains vccollector methods to cache vCenter entities
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vccollector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
)

const (
	StrAsterisk     = "*"
	StrErrorNotFoud = "'*' not found"
)

type hostState struct {
	notConnected   bool
	lastNoResponse time.Time
	notResponding  bool
}

type VcCache struct {
	lastDCUpdate time.Time                          //nolint
	lastCHUpdate time.Time                          //nolint
	lastDsUpdate time.Time                          //nolint
	lastNtUpdate time.Time                          //nolint
	dcs          []*object.Datacenter               //nolint
	clusters     [][]*object.ClusterComputeResource //nolint
	dss          [][]*object.Datastore              //nolint
	hosts        [][]*object.HostSystem             //nolint
	hostStates   [][]hostState                      //nolint
	nets         [][]object.NetworkReference        //nolint
}

func (c *VcCollector) getDatacenters(ctx context.Context) error {
	var err error

	if time.Since(c.lastDCUpdate) < c.dataDuration {
		return nil
	}

	finder := find.NewFinder(c.client.Client, false)
	if c.dcs, err = finder.DatacenterList(ctx, StrAsterisk); err != nil {
		return fmt.Errorf("Could not get datacenter list: %w", err)
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
		if c.clusters[i], err = finder.ClusterComputeResourceList(ctx, StrAsterisk); err != nil {
			if !strings.Contains(err.Error(), StrErrorNotFoud) {
				return fmt.Errorf("Could not get datacenter cluster list: %w", err)
			}
		}

		// hosts
		numhosts = len(c.hosts[i])
		if c.hosts[i], err = finder.HostSystemList(ctx, StrAsterisk); err != nil {
			return fmt.Errorf("Could not get datacenter node list: %w", err)
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

		if c.nets[i], err = finder.NetworkList(ctx, StrAsterisk); err != nil {
			if !strings.Contains(err.Error(), StrErrorNotFoud) {
				return fmt.Errorf("Could not get datacenter network list %w", err)
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

		if c.dss[i], err = finder.DatastoreList(ctx, StrAsterisk); err != nil {
			if !strings.Contains(err.Error(), StrErrorNotFoud) {
				return fmt.Errorf("Could not get datacenter datastore list %w", err)
			}
		}
	}
	c.lastDsUpdate = time.Now()

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

func (c *VcCollector) isHostConnectedRespondingIdx(dcindex, hostindex int) bool {
	var (
		connectedResponding bool
		hState              hostState
	)

	if len(c.hostStates) <= dcindex || len(c.hostStates[dcindex]) <= hostindex {
		return true
	}
	if len(c.hosts) <= dcindex || len(c.hosts[dcindex]) <= hostindex {
		return false
	}
	hState = c.hostStates[dcindex][hostindex]
	if !hState.notConnected {
		// limit notResponding in cache for skipNotRespondigFor
		if time.Since(hState.lastNoResponse) > c.skipNotRespondigFor {
			hState.notResponding = false
		}
		connectedResponding = !hState.notResponding
	}

	return connectedResponding
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

func (c *hostState) setNotConnected(conn bool) {
	c.notConnected = conn
}

func (c *hostState) setNotResponding(resp bool) {
	c.notResponding = resp
	if resp {
		c.lastNoResponse = time.Now()
	}
}
