// vcstat main package is a telegraf shim that allows vcstat to work as an execd input plugin
//  so you can monitor vCenter status and basic stats
//
// Author: Tesifonte Belda
// License: The MIT License (MIT)

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/influxdata/telegraf/plugins/common/shim"
	_ "github.com/tesibelda/vcstat/plugins/inputs/vcstat"
)

var pollInterval = flag.Duration("poll_interval", 60*time.Second, "how often to send metrics (default 1m)")
var configFile = flag.String("config", "", "path to the config file for this plugin")
var showVersion = flag.Bool("version", false, "show vcstat version and exit")
var Version string = ""
var err error

func main() {
	// parse command line options
	flag.Parse()
	if *showVersion {
		fmt.Println("vcstat", Version)
		os.Exit(0)
	}

	// create the shim. This is what will run your plugins.
	shim := shim.New()
	if shim==nil {
		fmt.Fprintf(os.Stderr, "Error creating telegraf shim\n")
		os.Exit(1)
	}

	// If no config is specified, all imported plugins are loaded.
	// otherwise follow what the config asks for.
	// Check for settings from a config toml file,
	// (or just use whatever plugins were imported above)
	err = shim.LoadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %s\n", err)
		os.Exit(1)
	}

	// run a single plugin until stdin closes or we receive a termination signal
	if err := shim.Run(*pollInterval); err != nil {
		fmt.Fprintf(os.Stderr, "Error running telegraf shim: %s\n", err)
		os.Exit(2)
	}
}
