# Telegraf execd vcstat input

vcstat is a VMware vSphere input plugin for Telegraf that gathers status and basic stats from vCenter using govmomi library (in a similar way to [govc *.info](https://github.com/vmware/govmomi/blob/master/govc/USAGE.md) commands). You may use this input in parallel with Telegraf's vsphere input to complement the performance metrics it collects. With vcstat input's data you may be able to detect when a node goes from green to red, an HBA goes from link-up to link-down or to know the number of ports used by a Distributed Virtual Switch.

# Compatibility

As of January 2022, realeases are built with a govmomi library version that supports vCenter 6.5, 6.7 and 7.0.
Use telegraf v1.14 or above so that execd input is available. 

# Configuration

* Download the [latest release package](https://github.com/tesibelda/vcstat/releases/latest) for your platform.

* Edit vcstat.conf file as needed. Example:

```toml
[[inputs.vcstat]]
  ## vCenter URL to be monitored and its credential
  vcenter = "https://vcenter.local/sdk"
  username = "user@corp.local"
  password = "secret"
  ## Use SSL but skip chain & host verification
  insecure_skip_verify = false

  #### you may enable or disable data collection per instance type ####
  ## collect cluster measurements (vcstat_cluster)
  # cluster_instances = true
  ## collect host status measurements (vcstat_host)
  # host_instances = true
  ## collect host firewall measurement (vcstat_host_firewall)
  # host_firewall_instances = false
  ## collect host bus adapter measurements (vcstat_host_hba)
  # host_hba_instances = false
  ## collect host network interface measurements (vcstat_host_nic)
  # host_nic_instances = false
  ## collect network distributed virtual switch measurements (vcstat_net_dvs)
  # net_dvs_instances = true
  ## collect network distributed virtual portgroup measurements (vcstat_net_dvp)
  # net_dvp_instances = false
```

* Edit telegraf's execd input configuration as needed. Example:

```
## Gather vSphere vCenter status and basic stats
[[inputs.execd]]
  command = ["/path/to/vcstat_binary", "-config", "/path/to/vcstat.conf"]
  signal = "none"
```

* Restart or reload Telegraf.

# Quick test in your environment

* Edit vcstat.conf file as needed (see above)

* Run vcstat with -config argument using that file.
```
/path/to/vcstat -config /path/to/vcstat.conf
```

* Wait for 1 minute or press enter. You should see lines like those in the Example output below.

# Metrics

- vcstat_vcenter
  - tags:
    - vcenter
  - fields:
    - name (string)
    - num_datacenters (int)
    - ostype (string)
    - version (string)
    - build (string)
- vcstat_datacenter
  - tags:
    - vcenter
    - dcname
  - fields:
    - num_clusters (int)
    - num_datastores (int)
    - num_hosts (int)
    - num_networks (int)
- vcstat_cluster
  - tags:
    - clustername
	- moid
    - vcenter
    - dcname
  - fields:
	- status (string)
	- status_code (int) 0-green, 1-gray, 2-yellow, 3-red
	- num_hosts (int)
	- num_effective_hosts (int)
	- num_cpu_cores (int)
	- num_cpu_threads (int)
	- total_cpu (int)
	- total_memory (int)
	- effective_cpu (int)
	- effective_memory (int)
- vcstat_host
  - tags:
    - esxhostname
	- moid
    - vcenter
    - dcname
  - fields:
	- status (string)
	- status_code (int) 0-green, 1-gray, 2-yellow, 3-red
	- reboot_required (int)
	- in_maintenance_mode (int)
	- connection_state (int)
	- connection_state_code (int)
- vcstat_host_firewall
  - tags:
    - esxhostname
    - vcenter
    - dcname
  - fields:
	- defaultaction (string)
	- enabled (bool)
	- loaded (bool)
- vcstat_host_hba
  - tags:
	- device
	- driver
    - esxhostname
    - vcenter
    - dcname
  - fields:
	- link_state (string) 0-link-up, 1-link-n/a, 2-unbound, 3-link-down
	- link_state_code (int)
- vcstat_host_nic
  - tags:
	- device
	- driver
    - esxhostname
    - vcenter
    - dcname
  - fields:
	- link_status (string)
	- link_status_code (int) 0-Up, 1-Unknown, 2-Down
	- admin_status (string)
	- duplex (string)
	- speed (int)
	- mac (string)
- vcstat_net_dvs
  - tags:
    - dvs
	- moid
    - vcenter
    - dcname
  - fields:
    - status (string)
    - status_code (int) 0-green, 1-gray, 2-yellow, 3-red
    - num_ports (int)
    - max_ports (int)
    - num_standalone_ports (int)
- vcstat_net_dvp
  - tags:
    - dvp
	- moid
    - vcenter
    - dcname
    - uplink (true/false)
  - fields:
    - status (string)
    - status_code (int) 0-green, 1-gray, 2-yellow, 3-red
    - num_ports (int)

# Example output

```plain
vcstat_vcenter,vcenter=vcenter.local name="VMware vCenter Server",num_datacenters=1i,ostype="linux-x64",version="6.5.0" 1639585700012818600
vcstat_datacenter,dcname=MyDC,moid=datacenter-2,vcenter=vcenter.local num_catastores=51i,num_hosts=8i,num_networks=32i,num_clusters=1i 1639585700181013900
vcstat_cluster,clustername=MyCluster-01,dcname=MyDC,moid=domain-c121,vcenter=vcenter.local num_cpu_cores=152i,total_cpu=342248i,total_memory=1648683421696i,effective_cpu=299032i,status="green",status_code=0i,num_hosts=8i,num_effective_hosts=8i,num_cpu_threads=304i,effective_memory=1502236i 1639585700198033100
vcstat_host,dcname=MyDC,esxhostname=myesxi01.local,moid=host-706,vcenter=vcenter.local connection_state_code=0i,status="green",status_code=0i,reboot_required=false,in_maintenance_mode=false,connection_state="connected" 1639585700602279400
vcstat_host_firewall,dcname=MyDC,esxhostname=myesxi01.local,vcenter=vcenter.local defaultaction="DROP",enabled=true,loaded=true 1639585701216566800
vcstat_host_hba,dcname=MyDC,device=vmhba0,driver=lpfc,esxhostname=myesxi01.local,vcenter=vcenter.local status="link-n/a",status_code=1i 1639585701216566800
vcstat_host_nic,dcname=MyDC,device=vmnic0,driver=ntg3,esxhostname=myesxi01.local,vcenter=vcenter.local link_status="Down",link_status_code=2i 1639585702275580100
vcstat_net_dvs,dcname=MyDC,dvs=DSwitch-E1,moid=dvs-e1,vcenter=vcenter.local num_standalone_ports=0i,status="green",status_code=0i,num_ports=421i,max_ports=2147483647i 1639585702303440200
vcstat_net_dvp,dcname=MyDC,dvp=DSwitch-E1-DVUplinks-e1,moid=dvportgroup-e1,uplink=true,vcenter=vcenter.local status="green",status_code=0i,num_ports=16i 1639585702303440200
```

# Build Instructions

Download the repo somewhere

    $ git clone git@github.com:tesibelda/vcstat.git

build the "vcstat" binary

    $ go build -o bin/vcstat cmd/main.go
    
 (if you're using windows, you'll want to give it an .exe extension)
 
    go build -o bin\vcstat.exe cmd/main.go

# Author

Tesifonte Belda (https://github.com/tesibelda)

# Contributing

Constructive contributions are welcome.

# License

[The MIT License (MIT)](https://github.com/tesibelda/vcstat/blob/master/LICENSE)
