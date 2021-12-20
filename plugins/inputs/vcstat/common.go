// common contains util functions for vcstat
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package vcstat

import (
	"github.com/vmware/govmomi/vim25/types"
)

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

// hostConnectionStateCode converts types.HostSystemConnectionState to int16 for easy alerting from telegraf metrics
func hostConnectionStateCode(state types.HostSystemConnectionState) int16 {
	switch state {
	case types.HostSystemConnectionStateConnected:
		return 0
	case types.HostSystemConnectionStateNotResponding:
		return 1
	case types.HostSystemConnectionStateDisconnected:
		return 2
	default:
		return 0
	}
}
