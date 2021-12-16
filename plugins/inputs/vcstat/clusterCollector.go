// clCollector gathers stats at cluster level
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vcstat

import (
	"context"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// clCollector type indicates succesfull cluster collection or not
type clCollector bool

// NewClCollector returns a new Collector exposing cluster stats.
func NewClCollector() (clCollector, error) {
	res := clCollector(false)
	return res, nil
}

// Collect gathers cluster info
func (c *clCollector) Collect(ctx context.Context, client *vim25.Client, dcs []*object.Datacenter, clMap map[int][]*object.ClusterComputeResource, acc telegraf.Accumulator) error {
	var clusters []*object.ClusterComputeResource
	var clMo mo.ClusterComputeResource
	var resourceSum *(types.ComputeResourceSummary)
	var err error = nil
	var clusterStatusCode int16 = 0

	for i, dc := range dcs {
		clusters = clMap[i]
		for _, cluster := range clusters {
			err = cluster.Properties(ctx, cluster.Reference(), []string{"summary"}, &clMo)
			if err != nil {
				*c = false
				return err
			}
			resourceSum = clMo.Summary.GetComputeResourceSummary()
			clusterStatusCode = entityStatusCode(resourceSum.OverallStatus)

			cltags := getClTags(client.URL().Host, dc.Name(), cluster.Name(), cluster.Reference().Value)
			clfields := getClFields(string(resourceSum.OverallStatus), clusterStatusCode, resourceSum.NumHosts, resourceSum.NumEffectiveHosts, resourceSum.NumCpuCores, resourceSum.NumCpuThreads, int(resourceSum.TotalCpu), int(resourceSum.TotalMemory), int(resourceSum.EffectiveCpu), int(resourceSum.EffectiveMemory))
			acc.AddFields("vcstat_cluster", clfields, cltags, time.Now())
		}
	}
	*c = true

	return nil
}

func getClTags(vcenter, dcname, clustername, moid string) map[string]string {
	return map[string]string{
		"vcenter":     vcenter,
		"dcname":      dcname,
		"clustername": clustername,
		"moid":        moid,
	}
}

func getClFields(overallstatus string, clusterstatuscode int16, numhosts, numeffectivehosts int32, numcpucores, numcputhreads int16, totalcpu, totalmemory, effectivecpu, effectivememory int) map[string]interface{} {
	return map[string]interface{}{
		"status":              overallstatus,
		"status_code":         clusterstatuscode,
		"num_hosts":           numhosts,
		"num_effective_hosts": numeffectivehosts,
		"num_cpu_cores":       numcpucores,
		"num_cpu_threads":     numcputhreads,
		"total_cpu":           totalcpu,
		"total_memory":        totalmemory,
		"effective_cpu":       effectivecpu,
		"effective_memory":    effectivememory,
	}
}
