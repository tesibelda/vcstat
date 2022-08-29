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
)

// CollectVcenterInfo gathers basic vcenter info
func (c *VcCollector) CollectVcenterInfo(
	ctx context.Context,
	acc telegraf.Accumulator,
) error {
	if c.client == nil {
		return fmt.Errorf("Could not get vcenter info: %w", Error_NoClient)
	}
	cli := c.client.Client

	if err := c.getDatacenters(ctx); err != nil {
		return err
	}

	vctags := getVcenterTags(cli.URL().Host)
	vcfields := getVcenterFields(
		cli.ServiceContent.About.Version,
		string(cli.ServiceContent.About.Build),
		cli.ServiceContent.About.Name,
		cli.ServiceContent.About.OsType,
		len(c.dcs),
	)
	acc.AddFields("vcstat_vcenter", vcfields, vctags, time.Now())

	return nil
}

func getVcenterTags(vcenter string) map[string]string {
	return map[string]string{
		"vcenter": vcenter,
	}
}

func getVcenterFields(
	version, build, name, ostype string,
	numdcs int,
) map[string]interface{} {
	return map[string]interface{}{
		"build":           build,
		"name":            name,
		"num_datacenters": numdcs,
		"ostype":          ostype,
		"version":         version,
	}
}
