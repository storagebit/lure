package main

import (
	"flag"
	"fmt"
	tm "github.com/buger/goterm"
	"github.com/dustin/go-humanize"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	go func() {
		http.HandleFunc("/lure", httpStats)
		log.Println(http.ListenAndServe("localhost:8666", nil))
	}()
}

var diskFilter = []string{"sda", "sda1"}

var diskStats = make(map[string]Stats)

type Stats struct {
	readOps    uint64
	writeOps   uint64
	readBytes  uint64
	writeBytes uint64
	resolution int
}

type DiskInfo struct {
	blocks    uint64
	blockSize int64
}

var diskBlockInfo = make(map[string]DiskInfo)

var interval = 1

func check(e error) {
	if e != nil {
		log.Fatalf("ERROR: %v", e)
	}
}

func Find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}

func main() {

	flag.IntVar(&interval, "interval", 1, "Sample interval in seconds, the default is 1.")

	flag.Parse()

	for _, disk := range diskFilter {

		diskDevPath := "/dev/" + disk

		var stat syscall.Statfs_t

		err := syscall.Statfs(diskDevPath, &stat)

		check(err)

		fmt.Println(disk, "block/sector size:", stat.Bsize)
		fmt.Println(disk, "blocks:", stat.Blocks)

		info := DiskInfo{}
		info.blocks = stat.Blocks
		info.blockSize = stat.Bsize

		diskBlockInfo[disk] = info
	}
	//fmt.Println(diskBlockInfo)

	//http.HandleFunc("/", returnStats)

	sampleStats()
}

func sampleStats() {

	timeInterval := time.Duration(interval) * time.Second

	var sampleCount = 0

	for {

		tm.Clear() // Clear current screen
		tm.MoveCursor(1, 1)
		currentTime := time.Now()
		strHeader := "Time: " + currentTime.String() + " | Sample Interval: " + strconv.Itoa(interval) + "s" + " | Samples: " + strconv.Itoa(sampleCount)
		_, _ = tm.Println(tm.Background(tm.Color(tm.Bold(strHeader), tm.BLACK), tm.GREEN))

		prevSample, err := ioutil.ReadFile("/proc/diskstats")

		time.Sleep(timeInterval)

		newSample, err := ioutil.ReadFile("/proc/diskstats")

		sampleCount++

		check(err)

		var slcPrevStats = strings.Split(string(prevSample), "\n")
		var slcNewStats = strings.Split(string(newSample), "\n")

		var prevReads uint64 = 0
		var prevWrites uint64 = 0
		var newReads uint64 = 0
		var newWrites uint64 = 0
		var prevReadSectors uint64 = 0
		var prevWriteSectors uint64 = 0
		var newReadSectors uint64 = 0
		var newWriteSectors uint64 = 0

		for _, device := range diskFilter {

			sampledStats := Stats{}

			for _, line := range slcPrevStats {

				if len(line) > 1 {

					var device = strings.Fields(line)[2]
					_, found := Find(diskFilter, device)

					if found {
						prevReads, _ = strconv.ParseUint(strings.Fields(line)[3], 10, 64)
						prevWrites, _ = strconv.ParseUint(strings.Fields(line)[7], 10, 64)
						prevReadSectors, _ = strconv.ParseUint(strings.Fields(line)[5], 10, 64)
						prevWriteSectors, _ = strconv.ParseUint(strings.Fields(line)[9], 10, 64)
					}
				}
			}
			for _, line := range slcNewStats {

				if len(line) > 1 {

					var device = strings.Fields(line)[2]
					_, found := Find(diskFilter, device)

					if found {
						newReads, _ = strconv.ParseUint(strings.Fields(line)[3], 10, 64)
						newWrites, _ = strconv.ParseUint(strings.Fields(line)[7], 10, 64)
						newReadSectors, _ = strconv.ParseUint(strings.Fields(line)[5], 10, 64)
						newWriteSectors, _ = strconv.ParseUint(strings.Fields(line)[9], 10, 64)
					}
				}
			}
			if newReads > prevReads {
				sampledStats.readOps = (newReads - prevReads) / uint64(interval)
			} else {
				sampledStats.readOps = 0
			}
			if newWrites > prevWrites {
				sampledStats.writeOps = (newWrites - prevWrites) / uint64(interval)
			} else {
				sampledStats.writeOps = 0
			}
			if newReadSectors > prevReadSectors {
				sampledStats.readBytes = (newReadSectors - prevReadSectors) * uint64(diskBlockInfo[device].blockSize) / uint64(interval)
			} else {
				sampledStats.readBytes = 0
			}
			if newWriteSectors > prevWriteSectors {
				sampledStats.writeBytes = (newWriteSectors - prevWriteSectors) * uint64(diskBlockInfo[device].blockSize) / uint64(interval)
			} else {
				sampledStats.writeBytes = 0
			}

			sampledStats.resolution = 1

			diskStats[device] = sampledStats
		}

		for key, device := range diskStats {
			_, _ = tm.Println(key, "\tRead Ops:", device.readOps, "\tWrite Ops:", device.writeOps, "\tRead Bytes:", humanize.Bytes(device.readBytes/8), "\tWrite Bytes:", humanize.Bytes(device.writeBytes/8))
		}

		tm.Flush()
	}

}

func httpStats(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprintln(w, "Hello Suckers!")
	for name, device := range diskStats {
		_, _ = fmt.Fprintf(w, name)
		_, _ = fmt.Fprintln(w, "\tRead Ops:", device.readOps, "\tWrite Ops:", device.writeOps, "\tRead Bytes:", humanize.Bytes(device.readBytes/8), "\tWrite Bytes:", humanize.Bytes(device.writeBytes/8))
	}
}
