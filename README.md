# Telegraf execd vcstat input

vcstat is a vSphere vCenter input plugin for Telegraf that gathers status and basic stats from vCenter using govmomi library (in a similar way as [gc *.info](https://github.com/vmware/govmomi/blob/master/govc/USAGE.md) commands). You may use this input in parallel with vsphere Telegraf's input to complement the performance metrics it collects. With vcstat input's data you may be able to detect when a node or HBA goes from green to red, or to know the number of ports used by a Distributed Virtual Switch.

# Configuration

* Download the [latest release package](https://github.com/tesibelda/vcstat/releases/latest) for your platform.

* Edit vcstat.conf file as needed. Example:

```toml
## Gather vSphere vCenter status and basic stats
[[inputs.vcstat]]
  vcenter = "https://vcenter.local/sdk"
  username = "user@corp.local"
  password = "secret"
  insecure_skip_verify = false

  ## you may enable or disable data collection per instance type
  cluster_instances = true
  host_instances = true
  # host_hba_instances = false
  # host_nic_instances = false
  net_dvs_instances = true
```

* Edit telegraf's execd input configuration as needed. Example:

```
[[inputs.execd]]
  command = ["/path/to/vcstat_binary", "-config", "/path/to/vcstat.conf"]
  signal = "none"
```

* Restart or reload Telegraf.

# Metrics

- vcstat_vcenter
  - tags:
    - vcenter
  - fields:
    - name (string)
    - num_datacenters (int)
    - ostype (string)
    - version (string)
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
	- status_code (int)
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
	- status_code (int)
	- reboot_required (int)
	- in_maintenance_mode (int)
	- connection_state (int)
	- connection_state_code (int)
- vcstat_host_hba
  - tags:
	- device
	- driver
    - esxhostname
    - vcenter
    - dcname
  - fields:
	- status (string)
	- status_code (int)
- vcstat_host_nic
  - tags:
	- device
	- driver
    - esxhostname
    - vcenter
    - dcname
  - fields:
	- status (string)
	- status_code (int)
- vcstat_net_dvs
  - tags:
    - dvs
	- moid
    - vcenter
    - dcname
  - fields:
    - status (string)
    - status_code (int)
    - num_ports (int)
    - max_ports (int)
    - num_standalone_ports (int)

# Example output

```plain
vcstat_vcenter,vcenter=vcenter.local name="VMware vCenter Server",num_datacenters=1i,ostype="linux-x64",version="6.5.0" 1639585700012818600
vcstat_datacenter,dcname=MyDC,moid=datacenter-2,vcenter=vcenter.local num_catastores=51i,num_hosts=8i,num_networks=32i,num_clusters=1i 1639585700181013900
vcstat_cluster,clustername=MyCluster-01,dcname=MyDC,moid=domain-c121,vcenter=vcenter.local num_cpu_cores=152i,total_cpu=342248i,total_memory=1648683421696i,effective_cpu=299032i,status="green",status_code=0i,num_hosts=8i,num_effective_hosts=8i,num_cpu_threads=304i,effective_memory=1502236i 1639585700198033100
vcstat_host,dcname=MyDC,esxhostname=myesxi01.local,moid=host-706,vcenter=vcenter.local connection_state_code=0i,status="green",status_code=0i,reboot_required=false,in_maintenance_mode=false,connection_state="connected" 1639585700602279400
vcstat_host_hba,dcname=MyDC,device=vmhba0,driver=lpfc,esxhostname=myesxi01.local,vcenter=vcenter.local status="link-n/a",status_code=1i 1639585701216566800
vcstat_host_nic,dcname=MyDC,device=vmnic0,driver=ntg3,esxhostname=myesxi01.local,vcenter=vcenter.local link_status="Down",link_status_code=2i 1639585702275580100
vcstat_net_dvs,dcname=MyDC,dvs=DSwitch-E1,moid=dvs-102,vcenter=vcenter.local num_standalone_ports=0i,status="green",status_code=0i,num_ports=421i,max_ports=2147483647i 1639585702303440200
```

# Build Instructions

Download the repo somewhere

    $ git clone git@github.com:tesibelda/vcstat.git

build the "vcstat" binary

    $ go build -o bin/vcstat cmd/main.go
    
 (if you're using windows, you'll want to give it an .exe extension)
 
    go build -o bin\vcstat.exe cmd/main.go
