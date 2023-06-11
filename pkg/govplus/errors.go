// govplus is a basic govmomi helper library for using vSphere API
//  This file contains error related functions and definitions
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package govplus

import (
	"context"
	"errors"
	"net"
)

// Common raised errors
var (
	ErrorNoClient   = errors.New("no vCenter client, no session has been opened")
	ErrorNotVC      = errors.New("endpoint does not look like a vCenter")
	ErrorURLParsing = errors.New("error parsing URL for vcenter")
	ErrorURLNil     = errors.New("vcenter URL should not be nil")
)

// IsHardQueryError returns false if error is light and we may continue quering
func IsHardQueryError(err error) (bool, error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true, err
	}

	var dnsError *net.DNSError
	var opError *net.OpError
	if errors.As(err, &dnsError) || errors.As(err, &opError) {
		return true, err
	}

	return false, err
}
