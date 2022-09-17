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

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// CollectClusterInfo gathers cluster info
func (c *VcCollector) CollectClusterInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		cltags      = make(map[string]string)
		clfields    = make(map[string]interface{})
		clMos       []mo.ClusterComputeResource
		arefs       []types.ManagedObjectReference
		resourceSum *(types.ClusterComputeResourceSummary)
		usageSum    *(types.ClusterUsageSummary)
		t           time.Time
		numVms      int32
		err         error
	)

	if c.client == nil || c.coll == nil {
		return fmt.Errorf("Could not get clusters info: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		// get cluster references and split the list into chunks
		for _, cluster := range c.clusters[i] {
			arefs = append(arefs, cluster.Reference())
		}
		chunks := chunckMoRefSlice(arefs, c.queryBulkSize)

		for _, refs := range chunks {
			err = c.coll.Retrieve(ctx, refs, []string{"name","summary"}, &clMos)
			if err != nil {
				if err, exit := govplus.IsHardQueryError(err); exit {
					return err
				}
				acc.AddError(
					fmt.Errorf(
						"Could not retrieve summary for cluster reference list: %w",
						err,
					),
				)
				continue
			}
			t = time.Now()

			for _, clMo := range clMos {
				resourceSum = clMo.Summary.(*types.ClusterComputeResourceSummary)
				if resourceSum == nil {
					return fmt.Errorf(
						"Could not get cluster resource summary for %s",
						clMo.Name,
					)
				}

				cltags["dcname"] = dc.Name()
				cltags["clustername"] = clMo.Name
				cltags["moid"] = clMo.Self.Reference().Value
				cltags["vcenter"] = c.client.Client.URL().Host

				// get number of VMs in the cluster
				// (ref: https://github.com/vmware/govmomi/issues/1247)
				numVms = 0
				if usageSum = resourceSum.UsageSummary; usageSum != nil {
					numVms = usageSum.TotalVmCount
				}
				clfields["effective_cpu"] = int64(resourceSum.EffectiveCpu)
				clfields["effective_memory"] = resourceSum.EffectiveMemory
				clfields["num_cpu_cores"] = resourceSum.NumCpuCores
				clfields["num_cpu_threads"] = resourceSum.NumCpuThreads
				clfields["num_effective_hosts"] = resourceSum.NumEffectiveHosts
				clfields["num_vms"] = numVms
				clfields["num_hosts"] = resourceSum.NumHosts
				clfields["status"] = string(resourceSum.OverallStatus)
				clfields["status_code"] = entityStatusCode(resourceSum.OverallStatus)
				clfields["total_cpu"] = int64(resourceSum.TotalCpu)
				clfields["total_memory"] = resourceSum.TotalMemory

				acc.AddFields("vcstat_cluster", clfields, cltags, t)
			}
		}
	}

	return nil
}
