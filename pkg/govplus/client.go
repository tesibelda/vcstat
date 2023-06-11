// govplus is a basic govmomi helper library for using vSphere API
//  This file contains API client related functions and definitions
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package govplus

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/influxdata/telegraf/plugins/common/tls"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
)

// NewClient creates a vSphere vim25.Client (ie inventory queries)
func NewClient(
	ctx context.Context,
	u *url.URL,
	t *tls.ClientConfig,
) (*govmomi.Client, error) {
	var (
		c   *govmomi.Client
		err error
	)

	if u == nil || u.User == nil {
		return nil, ErrorURLNil
	}

	if t.TLSCA == "" {
		c, err = govmomi.NewClient(ctx, u, t.InsecureSkipVerify)
	} else {
		c, err = newCAClient(ctx, u, t)
	}
	if err != nil {
		return nil, err
	}

	if !c.Client.IsVC() {
		return nil, ErrorNotVC
	}

	return c, nil
}

// NewRestClient creates a vSphere rest.Client (ie tags queries)
func NewRestClient(
	ctx context.Context,
	u *url.URL,
	t *tls.ClientConfig,
	c *govmomi.Client,
) (*rest.Client, error) {
	if c == nil || c.Client == nil {
		return nil, ErrorNoClient
	}
	// Share govc's session cache
	s := &cache.Session{
		URL:      u,
		Insecure: t.InsecureSkipVerify,
	}

	rc := rest.NewClient(c.Client)
	err := s.Login(ctx, rc, nil)
	if err != nil {
		return nil, err
	}

	return rc, nil
}

// CloseClient closes govmomi client
func CloseClient(ctx context.Context, c *govmomi.Client) {
	if c != nil {
		_ = c.Logout(ctx) //nolint: no worries for logout errors
	}
}

// CloseRestClient closes vSphere rest client
func CloseRestClient(ctx context.Context, rc *rest.Client) {
	if rc != nil {
		_ = rc.Logout(ctx) //nolint: no worries for logout errors
	}
}

// PaseURL parses vcenter URL params
func PaseURL(vcenterURL, user, pass string) (*url.URL, error) {
	u, err := soap.ParseURL(vcenterURL)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrorURLParsing.Error(), err)
	}
	if u == nil {
		return nil, fmt.Errorf("%w: returned nil", ErrorURLParsing)
	}
	u.User = url.UserPassword(user, pass)

	return u, nil
}

// ClientIsActive returns true if the vCenter soap session is active
func ClientIsActive(ctx context.Context, c *govmomi.Client) bool {
	if c == nil || !c.Client.Valid() {
		return false
	}

	ctx1, cancel1 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel1()
	_, err := methods.GetCurrentTime(ctx1, c.Client) //nolint no need current time

	return err == nil
}

// RestClientIsActive returns true if the vCenter rest session is active
func RestClientIsActive(ctx context.Context, rc *rest.Client) bool {
	if rc == nil {
		return false
	}

	s, err := rc.Session(ctx)
	if err != nil || s == nil {
		return false
	}

	return true
}

// newCAClient creates a Client but logins after setting CA, not before
func newCAClient(
	ctx context.Context,
	u *url.URL,
	t *tls.ClientConfig,
) (*govmomi.Client, error) {
	var err error

	soapClient := soap.NewClient(u, t.InsecureSkipVerify)
	if err = soapClient.SetRootCAs(t.TLSCA); err != nil {
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
	err = cli.Login(ctx, u.User)

	return cli, err
}
