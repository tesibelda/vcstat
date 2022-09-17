// tgplus is a basic telegraf helper library
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package tgplus

import (
	"context"
	"time"

	"github.com/influxdata/telegraf"
)

// GatherError adds the error to the telegraf accumulator
func GatherError(acc telegraf.Accumulator, err error) error {
	// No need to signal errors if we were merely canceled.
	if err == context.Canceled {
		return nil
	}
	acc.AddError(err)
	return nil
}

// GetPrecision returns the rounding precision for metrics
func GetPrecision(interval time.Duration) time.Duration {
	switch {
	case interval >= time.Second:
		return time.Second
	case interval >= time.Millisecond:
		return time.Millisecond
	case interval >= time.Microsecond:
		return time.Microsecond
	default:
		return time.Nanosecond
	}
}
