// vcCollector gathers stats at vCenter level
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vcstat

import (
	"context"
	"time"

	"github.com/influxdata/telegraf"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
)

// vcCollector struct contains kid resources for a vCenter
type vcCollector struct {
	dcs []*object.Datacenter
}

// NewVcCollector returns a new Collector exposing vCenter level stats
func NewVCCollector() (vcCollector, error) {
	res := vcCollector{
		dcs: nil,
	}
	return res, nil
}

// Collect gathers basic vcenter info
func (c *vcCollector) Collect(
	ctx context.Context,
	client *vim25.Client,
	acc telegraf.Accumulator,
) error {
	var err error
	finder := find.NewFinder(client, false)
	c.dcs, err = finder.DatacenterList(ctx, "*")
	if err != nil {
		return err
	}

	// vCenter info
	vctags := getVcenterTags(client.URL().Host)
	vcfields := getVcenterFields(
		client.ServiceContent.About.Version,
		string(client.ServiceContent.About.Build),
		client.ServiceContent.About.Name,
		client.ServiceContent.About.OsType,
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
