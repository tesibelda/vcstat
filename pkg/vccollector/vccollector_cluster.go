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
		clusters          []*object.ClusterComputeResource
		clMo              mo.ClusterComputeResource
		resourceSum       *(types.ComputeResourceSummary)
		clusterStatusCode int16
		err               error
	)

	if c.client == nil {
		return fmt.Errorf(string(Error_NoClient))
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		clusters = c.clusters[i]
		for _, cluster := range clusters {
			err = cluster.Properties(ctx, cluster.Reference(), []string{"summary"}, &clMo)
			if err != nil {
				return fmt.Errorf(
					"Could not get cluster %s summary property: %s",
					cluster.Name(),
					err,
				)
			}
			if resourceSum = clMo.Summary.GetComputeResourceSummary(); resourceSum == nil {
				return fmt.Errorf("Could not get cluster resource summary")
			}
			clusterStatusCode = entityStatusCode(resourceSum.OverallStatus)

			cltags := getClusterTags(
				c.client.Client.URL().Host,
				dc.Name(),
				cluster.Name(),
				cluster.Reference().Value,
			)
			clfields := getClusterFields(
				string(resourceSum.OverallStatus),
				clusterStatusCode,
				resourceSum.NumHosts,
				resourceSum.NumEffectiveHosts,
				resourceSum.NumCpuCores,
				resourceSum.NumCpuThreads,
				int(resourceSum.TotalCpu),
				int(resourceSum.TotalMemory),
				int(resourceSum.EffectiveCpu),
				int(resourceSum.EffectiveMemory),
			)
			acc.AddFields("vcstat_cluster", clfields, cltags, time.Now())
		}
	}

	return nil
}

func getClusterTags(vcenter, dcname, clustername, moid string) map[string]string {
	return map[string]string{
		"dcname":      dcname,
		"clustername": clustername,
		"moid":        moid,
		"vcenter":     vcenter,
	}
}

func getClusterFields(
	overallstatus string,
	clusterstatuscode int16,
	numhosts, numeffectivehosts int32,
	numcpucores, numcputhreads int16,
	totalcpu, totalmemory, effectivecpu, effectivememory int,
) map[string]interface{} {
	return map[string]interface{}{
		"effective_cpu":       effectivecpu,
		"effective_memory":    effectivememory,
		"num_cpu_cores":       numcpucores,
		"num_cpu_threads":     numcputhreads,
		"num_effective_hosts": numeffectivehosts,
		"num_hosts":           numhosts,
		"status":              overallstatus,
		"status_code":         clusterstatuscode,
		"total_cpu":           totalcpu,
		"total_memory":        totalmemory,
	}
}
