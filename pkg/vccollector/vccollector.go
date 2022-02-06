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
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// VcCollector struct contains session and entities of a vCenter
type VcCollector struct {
	tls.ClientConfig
	urlString  string
	url        *url.URL
	client     *govmomi.Client
	dcs        []*object.Datacenter
	clusters   [][]*object.ClusterComputeResource
	dss        [][]*object.Datastore
	hosts      [][]*object.HostSystem
	hostsRInfo [][]*types.HostRuntimeInfo
	nets       [][]object.NetworkReference
}

// Common errors raised by vccollector
const (
	Error_NoClient   = "No vCenter client, please open a session"
	Error_URLParsing = "Error parsing URL for vcenter"
	Error_NotVC      = "Endpoint does not look like a vCenter"
)

// NewVCCollector returns a new VcCollector associated with the provided vCenter URL
func NewVCCollector(
	ctx context.Context,
	vcenterUrl, user, pass string,
	clicfg *tls.ClientConfig,
) (*VcCollector, error) {
	var err error

	vcc := VcCollector{
		urlString: vcenterUrl,
	}
	vcc.TLSCA = clicfg.TLSCA
	vcc.InsecureSkipVerify = clicfg.InsecureSkipVerify

	// Parse URL params
	if vcc.url, err = soap.ParseURL(vcenterUrl); err != nil {
		return nil, fmt.Errorf(string(Error_URLParsing + ": %w" + err.Error()))
	}
	if vcc.url == nil {
		return nil, fmt.Errorf(string(Error_URLParsing + ": returned nil"))
	}
	vcc.url.User = url.UserPassword(user, pass)

	return &vcc, err
}

// Open opens a vCenter connection session or relogin if session already exists
func (c *VcCollector) Open(ctx context.Context) error {
	var err error

	// set a default 5s login timeout
	ctx1, cancel1 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel1()
	if c.client != nil {
		// Try to relogin and if not possible reopen session
		if err = c.client.Login(ctx1, c.url.User); err != nil {
			c.Close(ctx)
			if err = c.Open(ctx); err != nil {
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
		return fmt.Errorf(Error_NotVC)
	}

	return err
}

// IsOpen returns if the vCenter connection is active or not
func (c *VcCollector) IsActive(ctx context.Context) bool {
	var ok bool

	if c.client != nil {
		var err error
		if ok, err = c.client.SessionManager.SessionIsActive(ctx); err != nil {
			return false
		}
	}

	return ok
}

// Close closes vCenter connection
func (c *VcCollector) Close(ctx context.Context) {
	if c.client != nil {
		ctx1, cancel1 := context.WithTimeout(ctx, 5*time.Second)
		defer cancel1()
		c.client.Logout(ctx1) //nolint   //no need for logout error checking
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

// entityStatusCode converts types.ManagedEntityStatus to int16 for easy alerting from telegraf metrics
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

// govQueryError returns false if error is light and we may continue quering or not 
func govQueryError(err error) (error, bool) {
	if err == context.Canceled {
		return err, true
	}

	var dnsError *net.DNSError
	var opError *net.OpError
	if errors.As(err, &dnsError) || errors.As(err, &opError) {
		return err, true
	}

	return err, false
}