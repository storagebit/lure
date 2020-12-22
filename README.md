# lure
Lustre Filesystem Realtime Resource Explorer

## What is it?
lure is a tool allowing you to monitor lustre components statistics and status in realtime via a very simple to use commandline utility. 

It also provides the stats available via web browser interface.

## Whats new in this version?
- better support for daemonized operations
- reporting MDT and OST jobstats

## Note
For ldiskfs based lustre systems only. No plans and no desire to support ZFS specific stuff.

Also, I started this project as I was in need of a very simple to use tool which doesn't have 3rd party package or other software dependencies, can be easily distributed(just copy the binary to the server you want to look at) and doesn't require a PhD to get it built and working.
The code might look a bit clunky here and there in its first iteration, but it does the job for me, and I'll see where I can improve it, if required.
I'll also add more code documentation as I work on it and time allows.

## Current functionality:
### MDS Stats
- Report MDT metadata performance statistics
- Report MDT jobstats

### OST Stats
- Report OST throughput statistics
- Report OST jobstats

### Web interface
- Report MDT and OST performance statistics, incl. jobstats

## Stuff I'm working on for the next release
- clean up and streamline some code and evaluate if more parallelization is required
- add capacity and inode ldiskfs consumption reporting

## A bit further out
- export stats via http in JSON format(lower priority than jobstats)

## Installation
Quite simple actually. 
Just download the binary from here and run it on your MDS or OSS.

Or `git clone https://github.com/storagebit/lure/` and `cd` into the `bin` directory where you find the binary or build and compile it from the source using the `build_lure.sh` bash script in the `src` directory.
The choice is yours.

## How to use it
Also, quite simple.
```
$ ./lure -h
Usage of ./lure:
  -daemon
        Run as daemon in the background. No console output but stats available via web interface.
  -ignoremdt
        Don't report MDT stats.
  -ignoreost
        Don't report OST stats.
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

If you want to web access only, run lure in a fashion similar to: `nohup ./lure /dev/null 2>&1 &` - The upcoming functionality allowing it to run demonized in the background will make this obsolete.

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
