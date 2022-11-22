// This file contains vccollector methods to gather stats about graphics entities
//  (GPUs)
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

	"github.com/vmware/govmomi/govc/host/esxcli"
)

// CollectHostGraphics gathers host graphics device stats
func (c *VcCollector) CollectHostGraphics(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		grtags      = make(map[string]string)
		grfields    = make(map[string]interface{})
		x            *esxcli.Executor
		res          *esxcli.Response
		hostSt       *hostState
		startTime, t time.Time
		err          error
	)

	if c.client == nil || c.coll == nil {
		return fmt.Errorf("Could not get graphics device stats: %w", govplus.ErrorNoClient)
	}
	if err = c.getAllDatacentersClustersAndHosts(ctx); err != nil {
		return fmt.Errorf("Could not get cluster and host entity list: %w", err)
	}

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
			res, err = x.Run([]string{"graphics", "device", "stats", "list"})
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
			t = time.Now()

			if len(res.Values) > 0 {
				var keys []string
				for key := range res.Values[0] {
					keys = append(keys, key) //nolint
				}
				for _, rv := range res.Values {
					if len(rv) > 0 && len(rv["DeviceName"]) > 0 {
						grtags["clustername"] = c.getClusternameFromHost(i, host)
						grtags["dcname"] = dc.Name()
						grtags["address"] = rv["Address"][0]
						grtags["device"] = rv["DeviceName"][0]
						grtags["esxhostname"] = host.Name()
						grtags["vcenter"] = c.client.Client.URL().Host

						grfields["driver"] = rv["DriverVersion"][0]
						grfields["memory"] = rv["MemoryUsed"][0]
						grfields["temperature"] = rv["Temperature"][0]
						grfields["cpu"] = rv["Utilization"][0]

						acc.AddFields("vcstat_host_graphics", grfields, grtags, t)
					}
				}
			}
		}
	}

	return nil
}
