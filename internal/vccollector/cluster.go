// This file contains vccollector methods to gather stats about cluster entities
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vccollector

import (
	"context"
	"fmt"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/tesibelda/vcstat/pkg/govplus"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// CollectClusterInfo gathers cluster info
func (c *VcCollector) CollectClusterInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		clMo        mo.ClusterComputeResource
		cltags      map[string]string
		clfields    map[string]interface{}
		clusters    []*object.ClusterComputeResource
		resourceSum *(types.ClusterComputeResourceSummary)
		usageSum    *(types.ClusterUsageSummary)
		numVms      int32
		err         error
	)

	if c.client == nil {
		return fmt.Errorf("Could not get clusters info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

	// reserve map memory for tags and fields according to setClusterTags and setClusterFields
	cltags = make(map[string]string, 4)
	clfields = make(map[string]interface{}, 11)

	for i, dc := range c.dcs {
		clusters = c.clusters[i]
		for _, cluster := range clusters {
			err = cluster.Properties(ctx, cluster.Reference(), []string{"summary"}, &clMo)
			if err != nil {
				return fmt.Errorf(
					"Could not get cluster %s summary property: %w",
					cluster.Name(),
					err,
				)
			}
			if resourceSum = clMo.Summary.(*types.ClusterComputeResourceSummary); resourceSum == nil {
				return fmt.Errorf("Could not get cluster resource summary")
			}

			// get number of VMs in the cluster (tip: https://github.com/vmware/govmomi/issues/1247)
			numVms = 0
			usageSum = resourceSum.UsageSummary
			if usageSum != nil {
				numVms = usageSum.TotalVmCount
			}

			setClusterTags(
				cltags,
				c.client.Client.URL().Host,
				dc.Name(),
				cluster.Name(),
				cluster.Reference().Value,
			)
			setClusterFields(
				clfields,
				string(resourceSum.OverallStatus),
				entityStatusCode(resourceSum.OverallStatus),
				resourceSum.NumHosts,
				resourceSum.NumEffectiveHosts,
				resourceSum.NumCpuCores,
				resourceSum.NumCpuThreads,
				int64(resourceSum.TotalCpu),
				resourceSum.TotalMemory,
				int64(resourceSum.EffectiveCpu),
				resourceSum.EffectiveMemory,
				numVms,
			)
			acc.AddFields("vcstat_cluster", clfields, cltags, time.Now())
		}
	}

	return nil
}

func setClusterTags(
	tags map[string]string,
	vcenter, dcname, clustername, moid string,
) {
	tags["dcname"] = dcname
	tags["clustername"] = clustername
	tags["moid"] = moid
	tags["vcenter"] = vcenter
}

func setClusterFields(
	fields map[string]interface{},
	overallstatus string,
	clusterstatuscode int16,
	numhosts, numeffectivehosts int32,
	numcpucores, numcputhreads int16,
	totalcpu, totalmemory, effectivecpu, effectivememory int64,
	numvms int32,
) {
	fields["effective_cpu"] = effectivecpu
	fields["effective_memory"] = effectivememory
	fields["num_cpu_cores"] = numcpucores
	fields["num_cpu_threads"] = numcputhreads
	fields["num_effective_hosts"] = numeffectivehosts
	fields["num_vms"] = numvms
	fields["num_hosts"] = numhosts
	fields["status"] = overallstatus
	fields["status_code"] = clusterstatuscode
	fields["total_cpu"] = totalcpu
	fields["total_memory"] = totalmemory
}
