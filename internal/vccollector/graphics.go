// This file contains vccollector methods to gather stats about graphics entities
//  (GPUs)
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vccollector

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/tesibelda/vcstat/pkg/govplus"

	"github.com/vmware/govmomi/cli/esx"
)

// CollectHostGraphics gathers host graphics device stats
func (c *VcCollector) CollectHostGraphics(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		grtags       = make(map[string]string)
		grfields     = make(map[string]interface{})
		x            *esx.Executor
		res          *esx.Response
		hostSt       *hostState
		startTime, t time.Time
		err          error
	)

	if c.client == nil || c.coll == nil {
		return fmt.Errorf("could not get graphics device stats: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("could not get cluster and host entity list: %w", err)
	}

	for i, dc := range c.dcs {
		for j, host := range c.hosts[i] {
			if hostSt = c.getHostStateIdx(i, j); hostSt == nil {
				acc.AddError(fmt.Errorf("could not find host state idx entry for %s", host.Name()))
				continue
			}
			if !hostSt.isHostConnectedAndResponding(c.skipNotRespondigFor) {
				continue
			}
			startTime = time.Now()
			if x, err = esx.NewExecutor(ctx, c.client.Client, host); err != nil {
				hostExecutorNewAddError(acc, host.Name(), err)
				continue
			}
			res, err = x.Run(ctx, []string{"graphics", "device", "stats", "list"})
			hostSt.sumResponseTime(time.Since(startTime))
			if err != nil {
				hostExecutorRunAddError(acc, "graphics device", host.Name(), err)
				hostSt.setNotResponding(true)
				if exit, err := govplus.IsHardQueryError(err); exit {
					return err
				}
				continue
			}

			t = time.Now()
			for _, rv := range res.Values {
				if len(rv) > 0 && len(rv["DeviceName"]) > 0 {
					grtags["clustername"] = c.getClusternameFromHost(i, host)
					grtags["dcname"] = dc.Name()
					grtags["address"] = rv["Address"][0]
					grtags["device"] = rv["DeviceName"][0]
					grtags["esxhostname"] = host.Name()
					grtags["vcenter"] = c.client.Client.URL().Host

					grfields["driver"] = rv["DriverVersion"][0]
					grfields["memory"], _ = strconv.ParseFloat(rv["MemoryUsed"][0], 32)
					grfields["temperature"], _ = strconv.ParseFloat(rv["Temperature"][0], 32)
					grfields["cpu"], _ = strconv.ParseFloat(rv["Utilization"][0], 32)

					acc.AddFields("vcstat_host_graphics", grfields, grtags, t)
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
