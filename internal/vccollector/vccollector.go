// vccollector package allows you to gather basic stats from VMware vCenter using govmomi
//
//  Use NewVCCollector method to create a new struct, Open to open a session with a vCenter
// then use Collect* methods to get metrics added to a telegraf accumulator and finally
// Close when finished.
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vccollector

import (
	"context"
	"net/url"
	"time"

	"github.com/influxdata/telegraf/plugins/common/tls"

	"github.com/tesibelda/vcstat/pkg/govplus"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/types"
)

// VcCollector struct contains session and entities of a vCenter
type VcCollector struct {
	tls.ClientConfig
	urlString           string
	url                 *url.URL
	client              *govmomi.Client
	dataDuration        time.Duration
	skipNotRespondigFor time.Duration
	VcCache
}

// NewVCCollector returns a new VcCollector associated with the provided vCenter URL
func NewVCCollector(
	ctx context.Context,
	vcenterUrl, user, pass string,
	clicfg *tls.ClientConfig,
	dataDuration time.Duration,
) (*VcCollector, error) {
	var err error

	vcc := VcCollector{
		urlString:    vcenterUrl,
		dataDuration: dataDuration,
	}
	vcc.TLSCA = clicfg.TLSCA
	vcc.InsecureSkipVerify = clicfg.InsecureSkipVerify

	vcc.url, err = govplus.PaseURL(vcenterUrl, user, pass)
	if err != nil {
		return nil, err
	}

	return &vcc, err
}

// SetDataDuration sets max cache data duration
func (c *VcCollector) SetDataDuration(du time.Duration) error {
	c.dataDuration = du
	return nil
}

// SetSkipHostNotRespondingDuration sets time to skip not responding to esxcli commands hosts
func (c *VcCollector) SetSkipHostNotRespondingDuration(du time.Duration) error {
	c.skipNotRespondigFor = du
	return nil
}

// Open opens a vCenter connection session or relogin if session already exists
func (c *VcCollector) Open(ctx context.Context, timeout time.Duration) error {
	var err error

	// set a login timeout
	ctx1, cancel1 := context.WithTimeout(ctx, timeout)
	defer cancel1()
	if c.client != nil {
		// Try to relogin and if not possible reopen session
		if err = c.client.Login(ctx1, c.url.User); err == nil {
			return nil
		}
		c.Close(ctx)
	}
	c.client, err = govplus.NewClient(ctx1, c.url, &c.ClientConfig)

	return err
}

// IsActive returns if the vCenter connection is active or not
func (c *VcCollector) IsActive(ctx context.Context) bool {
	return govplus.ClientIsActive(ctx, c.client)
}

// Close closes vCenter connection
func (c *VcCollector) Close(ctx context.Context) {
	if c.client != nil {
		govplus.CloseClient(ctx, c.client)
		c.client = nil
	}
}

// entityStatusCode converts types.ManagedEntityStatus to int16 for easy alerting
func entityStatusCode(status types.ManagedEntityStatus) int16 {
	switch status {
	case types.ManagedEntityStatusGray:
		return 1
	case types.ManagedEntityStatusGreen:
		return 0
	case types.ManagedEntityStatusYellow:
		return 2
	case types.ManagedEntityStatusRed:
		return 3
	default:
		return 1
	}
}
