// This file contains vccollector methods to gathers stats about datacenters
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
	"github.com/vmware/govmomi/vim25/types"
)

const (
	StrAsterisk     = "*"
	StrErrorNotFoud = "'*' not found"
)

type VcCache struct {
	lastDCUpdate time.Time
	dcs          []*object.Datacenter

	lastUpdate time.Time
	clusters   [][]*object.ClusterComputeResource
	dss        [][]*object.Datastore
	hosts      [][]*object.HostSystem
	hostsRInfo [][]*types.HostRuntimeInfo
	nets       [][]object.NetworkReference
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

func (c *VcCollector) getAllDatacentersClustersAndHosts(
	ctx context.Context,
	moreEntities bool,
) error {
	if time.Since(c.lastUpdate) < c.dataDuration {
		return nil
	}
	err := c.getDatacenters(ctx)
	if err != nil {
		return err
	}

	numdcs := len(c.dcs)
	if numdcs != len(c.clusters) || numdcs != len(c.hosts) {
		if numdcs > 0 {
			c.clusters = make([][]*object.ClusterComputeResource, numdcs)
			c.hosts = make([][]*object.HostSystem, numdcs)
			c.hostsRInfo = make([][]*types.HostRuntimeInfo, numdcs)
		} else {
			c.clusters = nil
			c.hosts = nil
			c.hostsRInfo = nil
		}
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
		if c.hosts[i], err = finder.HostSystemList(ctx, StrAsterisk); err != nil {
			return fmt.Errorf("Could not get datacenter node list: %w", err)
		}
	}
	if !moreEntities {
		c.lastUpdate = time.Now()
	}

	return nil
}

func (c *VcCollector) getAllDatacentersNetworks(
	ctx context.Context,
	moreEntities bool,
) error {
	if time.Since(c.lastUpdate) < c.dataDuration {
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
	if !moreEntities {
		c.lastUpdate = time.Now()
	}

	return nil
}

func (c *VcCollector) getAllDatacentersDatastores(
	ctx context.Context,
	moreEntities bool,
) error {
	if time.Since(c.lastUpdate) < c.dataDuration {
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
	if !moreEntities {
		c.lastUpdate = time.Now()
	}

	return nil
}
