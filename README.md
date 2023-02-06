# Telegraf execd vcstat input

vcstat is a VMware vSphere input plugin for [Telegraf](https://github.com/influxdata/telegraf) that gathers status and basic [stats](https://github.com/tesibelda/vcstat/blob/master/METRICS.md) from vCenter using govmomi library (in a similar way to [govc *.info](https://github.com/vmware/govmomi/blob/master/govc/USAGE.md) commands). You may use this input in parallel with Telegraf's vsphere input to complement the performance metrics it collects. With vcstat input's data you may be able to detect when a node goes from green to red, an HBA goes from link-up to link-down, to know the number of ports used by a Distributed Virtual Switch or create a basic capacity dashboard.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/tesibelda/vcstat/raw/master/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/tesibelda/vcstat)](https://goreportcard.com/report/github.com/tesibelda/vcstat)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/tesibelda/vcstat?display_name=release)

# Compatibility

Latest releases are built with a govmomi library version that supports vCenter 7.0 and 8.0 (probably also work with 6.5 and 6.7).
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
  ## requests timeout. Here 0s is interpreted as the polling interval
  # timeout = "0s"
  ## Max number of objects to gather per query
  # query_bulk_size = 100
  ## number of intervals to skip esxcli commands for not responding hosts
  # intervals_skip_notresponding_esxcli_hosts = 20

  ## Optional SSL Config
  # tls_ca = "/path/to/cafile"
  ## Use SSL but skip chain & host verification
  # insecure_skip_verify = false

  ## Filter clusters by name, default is no filtering
  ## cluster names can be specified as glob patterns
  # clusters_include = []
  # clusters_exclude = []

  ## Filter hosts by name, default is no filtering
  ## host names can be specified as glob patterns
  # hosts_include = []
  # hosts_exclude = []

  ## Filter VMs by name, default is no filtering
  ## VM names can be specified as glob patterns
  # vms_include = []
  # vms_exclude = []

  #### you may enable or disable data collection per instance type ####
  ## collect cluster measurement (vcstat_cluster)
  # cluster_instances = true
  ## collect datastore measurement (vcstat_datastore)
  # datastore_instances = false
  ## collect host status measurement (vcstat_host)
  # host_instances = true
  ## collect host firewall measurement (vcstat_host_firewall)
  # host_firewall_instances = false
  ## collect host graphics measurement (vcstat_host_graphics)
  # host_graphics_instances = false
  ## collect host bus adapter measurement (vcstat_host_hba)
  # host_hba_instances = false
  ## collect host network interface measurement (vcstat_host_nic)
  # host_nic_instances = false
  ## collect host services measurement (vcstat_host_service)
  # host_service_instances = false
  ## collect network distributed virtual switch measurement (vcstat_net_dvs)
  # net_dvs_instances = true
  ## collect network distributed virtual portgroup measurement (vcstat_net_dvp)
  # net_dvp_instances = false
  ## collect virtual machine measurement (vcstat_vm)
  # vm_instances = false
```

* Edit telegraf's execd input configuration as needed. Example:

```
## Gather vSphere vCenter status and basic stats
[[inputs.execd]]
  command = ["/path/to/vcstat_binary", "-config", "/path/to/vcstat.conf"]
  signal = "none"
```

You can optionally tell vcstat the input's interval by adding -poll_interval the_interval parameters to the command. By default it expects 1m interval. If you want 30s interval configure it like this:
```
## Gather vSphere vCenter status and basic stats
[[inputs.execd]]
  interval = "30s"
  command = ["/path/to/vcstat_binary", "-config", "/path/to/vcstat.conf", "-poll_interval", "30s"]
  signal = "none"
```
Metric timestamp precision will be set according to the polling interval, so it will usually be 1s.

* Restart or reload Telegraf.

# Quick test in your environment

* Edit vcstat.conf file as needed (see above)

* Run vcstat with --config argument using that file.
```
/path/to/vcstat --config /path/to/vcstat.conf
```

* Wait for 1 minute or press enter. You should see lines like those in the Example output below.

* Note that vcstat will escape querying not connected hosts and also skip hosts for intervals_skip_notresponding_esxcli_hosts intervals if they don't respond to esxcli commands

# Example output

```plain
vcstat_vcenter,vcenter=vcenter.local name="VMware vCenter Server",num_datacenters=1i,ostype="linux-x64",version="6.5.0" 1653060681000000000
vcstat_datacenter,dcname=MyDC,moid=datacenter-2,vcenter=vcenter.local num_datastores=51i,num_hosts=8i,num_networks=32i,num_clusters=1i 1653060681000000000
vcstat_cluster,clustername=MyCluster-01,dcname=MyDC,moid=domain-c121,vcenter=vcenter.local num_cpu_cores=152i,total_cpu=342248i,total_memory=1648683421696i,effective_cpu=299032i,status="green",status_code=0i,num_vms=26i,num_hosts=8i,num_effective_hosts=8i,num_cpu_threads=304i,effective_memory=1502236i 1653060681000000000
vcstat_host,dcname=MyDC,clustername=MyCluster-01,esxhostname=myesxi01.local,moid=host-706,vcenter=vcenter.local connection_state_code=0i,memory_size=206110695424i,num_cpus=16i,cpu_freq=2199i,status="green",status_code=0i,reboot_required=false,in_maintenance_mode=false,connection_state="connected" 1653060681000000000
vcstat_host_graphics,address=0000:3b:00.0,clustername=MyCluster-01,dcname=MyDC,device=NVIDIA\ A40,esxhostname=myesxi01.local,vcenter=vcenter.local memory="9",temperature="29",cpu="3",driver="510.84.01" 1653060681000000000
vcstat_host_graphics,address=0000:a1:00.0,clustername=MyCluster-01,dcname=MyDC,device=NVIDIA\ A40,esxhostname=myesxi01.local,vcenter=vcenter.local driver="510.84.01",memory="11",temperature="28",cpu="5" 1653060681000000000
vcstat_host_firewall,dcname=MyDC,clustername=MyCluster-01,esxhostname=myesxi01.local,vcenter=vcenter.local defaultaction="DROP",enabled=true,loaded=true 1653060681000000000
vcstat_host_hba,dcname=MyDC,clustername=MyCluster-01,device=vmhba0,driver=lpfc,esxhostname=myesxi01.local,vcenter=vcenter.local status="link-n/a",status_code=1i 1653060681000000000
vcstat_host_nic,dcname=MyDC,clustername=MyCluster-01,device=vmnic0,driver=ntg3,esxhostname=myesxi01.local,vcenter=vcenter.local link_status="Down",link_status_code=2i 1653060681000000000
vcstat_host_esxcli,dcname=MyDC,clustername=MyCluster-01,esxhostname=myesxi01.local,moid=host-706,vcenter=vcenter.local  responding_code=0i,response_time_ns=109185876i 1653060681000000000
vcstat_host_service,dcname=MyDC,clustername=MyCluster-01,esxhostname=myesxi01.local,key=ntpd,vcenter=vcenter.local label="NTP Daemon",policy="on",required=false,running=true 1653060681000000000
vcstat_host_service,dcname=MyDC,clustername=MyCluster-01,esxhostname=myesxi01.local,key=vpxa,vcenter=vcenter.local label="VMware vCenter Agent",policy="on",required=false,running=true 1653060681000000000
vcstat_net_dvs,dcname=MyDC,dvs=DSwitch-E1,moid=dvs-e1,vcenter=vcenter.local num_standalone_ports=0i,status="green",status_code=0i,num_ports=421i,max_ports=2147483647i 1653060682000000000
vcstat_net_dvp,dcname=MyDC,dvp=DSwitch-E1-DVUplinks-e1,moid=dvportgroup-e1,uplink=true,vcenter=vcenter.local status="green",status_code=0i,num_ports=16i 1653060682000000000
vcstat_datastore,dcname=MyDC,dsname=DS_Departement1,moid=datastore-725,type=VMFS,vcenter=vcenter.local accessible=true,capacity=2198754820096i,freespace=730054262784i,uncommitted=20511i,maintenance_mode="normal" 
vcstat_vm,clustername=MyCluster-01,dcname=MyDC,esxhostname=myesxi01.local,moid=vm-4524,vcenter.local,vmname=vmserver01 status="green",status_code=0i,consolidation_needed=false,max_cpu_usage=11972i,num_eth_cards=1i,num_vdisks=2i,connection_state_code=0i,max_mem_usage=8589934592i,num_vcpus=4i,power_state_code=0i,template=false,connection_state="connected",memory_size=8589934592i,power_state="poweredOn" 1653060683000000000
internal_vcstat,vcenter=vcenter.local sessions_created=1i,gather_time_ns=1764839000i,notresponding_esxcli_hosts=0i 1653060683000000000
```

# Metrics
See [Metrics](https://github.com/tesibelda/vcstat/blob/master/METRICS.md)

# Build Instructions

Download the repo

    $ git clone git@github.com:tesibelda/vcstat.git

build the "vcstat" binary

    $ go build -o bin/vcstat cmd/main.go
    
 (if you're using windows, you'll want to give it an .exe extension)
 
    $ go build -o bin\vcstat.exe cmd/main.go

 If you use [go-task](https://github.com/go-task/task) execute one of these
 
    $ task linux:build
	$ task windows:build

# Author

Tesifonte Belda (https://github.com/tesibelda)

# Support and assitance

Reach out to the maintainer at one of the following places:

- [GitHub issues](https://github.com/tesibelda/vcstat/issues)
- Contact options listed on this GitHub profile

If you want to say **thank you** or/and support active development of vcstat:

- Add a [GitHub Star](https://github.com/tesibelda/vcstat) to the project.
- Write interesting articles about the project on [Dev.to](https://dev.to/), [Medium](https://medium.com/) or your personal blog.

# Disclaimer

The author and maintainers are not affiliated with VMware.
VMware is a registered trademark or trademark of VMware, Inc.

# License

[The MIT License (MIT)](https://github.com/tesibelda/vcstat/blob/master/LICENSE)
