// vccollector package allows you to gather basic stats from VMware vCenter using govmomi
//
//  Use New method to create a new struct, Open to open a session with a vCenter and then
// use Collect* methods to get metrics added to a telegraf accumulator and finally
// Close when finished.
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vccollector

import (
	"context"
	"net/url"
	"time"

	"github.com/influxdata/telegraf/filter"
	"github.com/influxdata/telegraf/plugins/common/tls"

	"github.com/tesibelda/vcstat/pkg/govplus"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/types"
)

// VcCollector struct contains session and entities of a vCenter
type VcCollector struct {
	tls.ClientConfig
	urlString           string
	url                 *url.URL
	client              *govmomi.Client
	coll                *property.Collector
	filterClusters      filter.Filter
	filterHosts         filter.Filter
	filterVms           filter.Filter
	maxResponseDuration time.Duration
	dataDuration        time.Duration
	skipNotRespondigFor time.Duration
	queryBulkSize       int
	VcCache
}

// New returns a new VcCollector associated with the provided vCenter URL
func New(
	vcenterURL, user, pass string,
	clicfg *tls.ClientConfig,
	dataDuration time.Duration,
) (*VcCollector, error) {
	var err error

	vcc := VcCollector{
		urlString:    vcenterURL,
		dataDuration: dataDuration,
	}
	if err = vcc.SetFilterClusters(nil, nil); err != nil {
		return nil, err
	}
	if err = vcc.SetFilterHosts(nil, nil); err != nil {
		return nil, err
	}
	if err = vcc.SetFilterVms(nil, nil); err != nil {
		return nil, err
	}
	vcc.TLSCA = clicfg.TLSCA
	vcc.InsecureSkipVerify = clicfg.InsecureSkipVerify

	vcc.url, err = govplus.PaseURL(vcenterURL, user, pass)
	if err != nil {
		return nil, err
	}

	return &vcc, err
}

// SetDataDuration sets max cache data duration
func (c *VcCollector) SetDataDuration(du time.Duration) {
	c.dataDuration = du
}

// SetFilterClusters sets clusters include and exclude filters
func (c *VcCollector) SetFilterClusters(include []string, exclude []string) error {
	var err error

	c.filterClusters, err = filter.NewIncludeExcludeFilter(include, exclude)
	if err != nil {
		return err
	}
	return nil
}

// SetFilterHosts sets hosts include and exclude filters
func (c *VcCollector) SetFilterHosts(include []string, exclude []string) error {
	var err error

	c.filterHosts, err = filter.NewIncludeExcludeFilter(include, exclude)
	if err != nil {
		return err
	}
	return nil
}

// SetFilterVms sets VMs include and exclude filters
func (c *VcCollector) SetFilterVms(include []string, exclude []string) error {
	var err error

	c.filterVms, err = filter.NewIncludeExcludeFilter(include, exclude)
	if err != nil {
		return err
	}
	return nil
}

// SetMaxResponseTime sets max response time to consider an esxcli command as notresponding
func (c *VcCollector) SetMaxResponseTime(du time.Duration) {
	c.maxResponseDuration = du
}

// SetQueryChunkSize sets chunk size of slice to use in sSphere property queries
func (c *VcCollector) SetQueryChunkSize(b int) {
	c.queryBulkSize = b
}

// SetSkipHostNotRespondingDuration sets time to skip not responding to esxcli commands hosts
func (c *VcCollector) SetSkipHostNotRespondingDuration(du time.Duration) {
	c.skipNotRespondigFor = du
}

// Open opens a vCenter connection session or relogin if session already exists
func (c *VcCollector) Open(timeout time.Duration) error {
	var err error

	// set a login timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if c.client != nil {
		// Try to relogin and if not possible reopen session
		if err = c.client.Login(ctx, c.url.User); err == nil {
			return nil
		}
		c.Close()
	}
	c.client, err = govplus.NewClient(ctx, c.url, &c.ClientConfig)
	if err == nil {
		c.coll = property.DefaultCollector(c.client.Client)
	}

	return err
}

// IsActive returns if the vCenter connection is active or not
func (c *VcCollector) IsActive(ctx context.Context) bool {
	return govplus.ClientIsActive(ctx, c.client)
}

// Close closes vCenter connection
func (c *VcCollector) Close() {
	if c.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		_ = c.coll.Destroy(ctx) //nolint: destroy and forget old collector
		govplus.CloseClient(ctx, c.client)
		c.client, c.coll = nil, nil
		cancel()
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

// chuckMoRefSlice returns a list of lists segregating a list of oManagedObjectReference
//
//	into chunks with a size of chunkSize
func chunckMoRefSlice(
	fList []types.ManagedObjectReference,
	chunkSize int,
) [][]types.ManagedObjectReference {
	var (
		chunks          [][]types.ManagedObjectReference
		listLen, end, i int
	)

	listLen = len(fList)

	for ; i < listLen; i += chunkSize {
		end = i + chunkSize
		if end > listLen {
			end = listLen
		}

		chunks = append(chunks, fList[i:end])
	}

	return chunks
}
