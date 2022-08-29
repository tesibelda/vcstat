// vccollector package allows you to gather basic stats from VMware vCenter using govmomi library
//
//  Use NewVCCollector method to create a new struct, Open to open a session with a vCenter then
// use Collect* methods to get metrics added to a telegraf accumulator and finally Close when
// finished.
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vccollector

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/influxdata/telegraf/plugins/common/tls"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
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

// Common errors raised by vccollector
var (
	Error_NoClient = errors.New("no vCenter client, no session has been opened")
	Error_NotVC    = errors.New("endpoint does not look like a vCenter")
)

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

	// Parse URL params
	if vcc.url, err = soap.ParseURL(vcenterUrl); err != nil {
		return nil, fmt.Errorf("Error parsing URL for vcenter: %w", err)
	}
	if vcc.url == nil {
		return nil, fmt.Errorf("Error parsing URL for vcenter: returned nil")
	}
	vcc.url.User = url.UserPassword(user, pass)

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
		if err = c.client.Login(ctx1, c.url.User); err != nil {
			c.Close(ctx)
			if err = c.Open(ctx, timeout); err != nil {
				return err
			}
		}
	} else {
		var cli *govmomi.Client

		// Create a vSphere vCenter client using CA if provided
		if c.TLSCA == "" {
			cli, err = govmomi.NewClient(ctx1, c.url, c.InsecureSkipVerify)
		} else {
			cli, err = c.newCAClient(ctx1)
		}
		if err != nil {
			return err
		}

		c.client = cli
	}
	if !c.client.IsVC() {
		c.Close(ctx)
		return fmt.Errorf("Could not open vCenter session: %w", Error_NotVC)
	}

	return err
}

// IsActive returns if the vCenter connection is active or not
func (c *VcCollector) IsActive(ctx context.Context) bool {
	if c == nil  || c.client == nil || !c.client.Valid() {
		return false
	}

	ctx1, cancel1 := context.WithTimeout(ctx, time.Duration(5 * time.Second))
	defer cancel1()
	_, err := methods.GetCurrentTime(ctx1, c.client) //nolint no need current time

	return err == nil
}

// Close closes vCenter connection
func (c *VcCollector) Close(ctx context.Context) {
	if c.client != nil {
		c.client.Logout(ctx) //nolint   //no need for logout error checking
		c.client = nil
	}
}

// newCAClient creates a Client but logins after setting CA, not before
func (c *VcCollector) newCAClient(ctx context.Context) (*govmomi.Client, error) {
	var err error

	soapClient := soap.NewClient(c.url, c.InsecureSkipVerify)
	if err = soapClient.SetRootCAs(c.TLSCA); err != nil {
		return nil, err
	}
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, err
	}

	cli := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}
	err = cli.Login(ctx, c.url.User)

	return cli, err
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

// govQueryError returns false if error is light and we may continue quering
func govQueryError(err error) (error, bool) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err, true
	}

	var dnsError *net.DNSError
	var opError *net.OpError
	if errors.As(err, &dnsError) || errors.As(err, &opError) {
		return err, true
	}

	return err, false
}
