# lure
Lustre Filesystem Realtime Resource Explorer

## What is it?
One single binary does it all! lure is a tool allowing you to monitor lustre clients, OSS and MDS serversin realtime with a very simple to use commandline utility.

It also provides the stats available via web browser interface and supports a direct feed server and client statistics into an InfluxDB.

## What's new in this version?
- report Lustre client performance stats
 

## Note
Developed on an ldiskfs based lustre system. Should work on ZFS based systems too, let me know!

Also, I started this project as I was in need of a very simple to use tool which doesn't have 3rd party package or other software dependencies, can be easily distributed(just copy the binary to the server you want to look at) and doesn't require a PhD to get it built and working.
The code might look a bit clunky here and there in its first iteration, but it does the job for me, and I'll see where I can improve it, if required.
I'll also add more code documentation as I work on it and time allows.

## Current functionality:
### Lustre client Stats
- Report throughput and metadata statistics

### MDS Stats
- Report MDT metadata performance statistics
- Report MDT jobstats

### OST Stats
- Report OST throughput statistics
- Report OST jobstats

### Web/JSON interface
- Report client, MDT and OST performance statistics, incl. jobstats
- All stats can be pulled via HTTP Get. Details further down

### InfluxDB support
- feed all client,  MDT and OST stats and jobstats directly into an InfluxDB
- support for InfluxDB 1.8 or 2.x or later
- Note: You can use this tool with Prometheus too, just use the Prometheus json exporter.

## Stuff I'm working on for the next release
- Continuous code clean up
- Report OS, MDT and client IOPs details 
- Add capacity and inode ldiskfs consumption reporting

## Installation
Quite simple actually. 
Just download the binary from here and run it on your Lustre client, MDS or OSS.

Or `git clone https://github.com/storagebit/lure/` and `cd` into the `bin` directory where you find the binary or build and compile it from the source using the `build_lure.sh` bash script in the `src` directory.
The choice is yours.

## How to use it
Also, quite simple.
```
$ ./lure -h
Usage of ./lure:
  -daemon
    	Run as daemon in the background. No console output but stats available via web interface.
  -feedtoinflux
    	Store statistics in InfluxDB
  -ignore
    	Don't report OST stats.
  -ignoremdt
    	Don't report MDT stats.
  -influxbucket string
    	InfluxDB bucket (default "lure")
  -influxorg string
    	InfluxDB org (default "storagebit")
  -influxport string
    	InfluxDB server port (default "8086")
  -influxserver string
    	InfluxDB server name or IP (default "localhost")
  -influxtoken string
    	Read/Write token for the bucket or user:password in the InfluxDB (default "lure:password")
  -interval int
    	Sample interval in seconds (default 1)
  -jobstats
    	Report Lustre Jobstats for MDT and OST devices.
  -port int
    	HTTP port used to access the the stats via web browser. (default 8666)
  -version
    	Print version information.
```
To access the stats via web browser use: `http://<ip address>:<port number>/stats` as the URL

If you want to web access only, run lure in a fashion similar to: `nohup ./lure -daemon /dev/null 2>&1 &` Or you can also write a systemd unit file and run it as a lightweight daemon or service. That's how I run it.

## Sample command line output(web will look very similar)
```
MDT Metadata Stats /s:
         Device      open     close     mknod      link    unlink     mkdir     rmdir    rename   getattr   setattr  getxattr  setxattr    statfs      sync  s_rename  c_rename
  testfs-MDT0001         0         0         0         0         0         0         0         0         0         0         0         0         1         0         0         0
  testfs-MDT0000         0         0         0         0         0         0         0         0         0         0         0         0         1         0         0         0

OST Operation Stats /s:
         Device  write_bytes   read_bytes      setattr       statfs       create      destroy        punch         sync     get_info     set_info
  testfs-OST0001            0            0            0            2            0            0            0            0            0            0
  testfs-OST0000            0            0            0            2            0            0            0            0            0            0
```
## Stats in JSON format via HTTP Get
- Lustre client stats via HTTP Get at `http://<ip address>:<port number>/json?stats=client`
- MDT stats via HTTP Get at `http://<ip address>:<port number>/json?stats=mdt`
- OST stats via HTTP Get at `http://<ip address>:<port number>/json?stats=ost`
- MDT Jobstats via HTTP Get at `http://<ip address>:<port number>/json?stats=mdtjob`
- OST Jobstats via HTTP Get at `http://<ip address>:<port number>/json?stats=ostjob`

Returns HTTP status 204 if there is no data to display, HTTP status 500 if there is an internal error, or HTTP status 400 if the request/URL was incorrect.

## Note on InfluxDB
- lure supports v1.8+ and the new InfluxDB format as introduced with version 2.x+
- If you use v1.8+, as I do mostly, create the DB manually and setup user credentials with read/write access for the DB
- For v1.8+, use the database name or the database/retention_policy name as "bucket" and the user:password for the token

## Note on the example Grafana dashboard
- setup the InfluxDB data source as v1 InfluxDB connection
- Don't forget to match the sample interval to the lure interval

## Happy Lustre Real-Time Monitoring!
