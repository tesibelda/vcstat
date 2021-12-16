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

// hbaStatusCode converts HostHostBusAdapter Status to int16 for easy alerting from telegraf metrics
func hbaStatusCode(state string) int16 {
	switch state {
	case "online":
		return 0
	case "unknown":
		return 1
	case "offline":
		return 2
	default:
		return 1
	}
}

// hbaLinkStateCode converts storage adapter Link State to int16 for easy alerting from telegraf metrics
func hbaLinkStateCode(state string) int16 {
	switch state {
	case "link-up":
		return 0
	case "link-n/a":
		return 1
	case "link-down":
		return 2
	default:
		return 1
	}
}

// nicLinkStatusCode converts LinkStatus to int16 for easy alerting from telegraf metrics
func nicLinkStatusCode(state string) int16 {
	switch state {
	case "Up":
		return 0
	case "Unknown":
		return 1
	case "Down":
		return 2
	default:
		return 1
	}
}
