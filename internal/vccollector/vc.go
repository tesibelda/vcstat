// This file contains vccollector methods to gathers stats at vcenter level
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

	"github.com/vmware/govmomi/vim25"
)

// CollectVcenterInfo gathers basic vcenter info
func (c *VcCollector) CollectVcenterInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	var (
		vctags   = make(map[string]string)
		vcfields = make(map[string]interface{})
		cli      *vim25.Client
		t        time.Time
		err      error
	)

	if c.client == nil {
		return fmt.Errorf("could not get vcenter info: %w", govplus.ErrorNoClient)
	}
	cli = c.client.Client

	if err = c.getDatacenters(ctx); err != nil {
		return err
	}
	t = time.Now()

	vctags["vcenter"] = cli.URL().Host

	vcfields["build"] = string(cli.ServiceContent.About.Build)
	vcfields["name"] = cli.ServiceContent.About.Name
	vcfields["num_datacenters"] = len(c.dcs)
	vcfields["ostype"] = cli.ServiceContent.About.OsType
	vcfields["version"] = cli.ServiceContent.About.Version

	acc.AddFields("vcstat_vcenter", vcfields, vctags, t)

	return nil
}
