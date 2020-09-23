package main

import (
	"flag"
	"fmt"
	tm "github.com/buger/goterm"
	"github.com/dustin/go-humanize"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	interval        uint64
	mapMDTCalcStats = make(map[string]map[string]uint64)
	mapOSTCalcStats = make(map[string]map[string]uint64)
	pathToMdts      = "/proc/fs/lustre/mdt"
	pathToOSTs      = "/proc/fs/lustre/obdfilter"
	mapMDTs         = make(map[string]string)
	mapOSTs         = make(map[string]string)
	mdtCounters     = []string{"open", "close", "mknod", "link", "unlink", "mkdir", "rmdir", "rename", "getattr",
		"setattr", "getxattr", "setxattr", "statfs", "sync", "s_rename", "c_rename"}
	ostCounters = []string{"write_bytes", "read_bytes", "setattr", "statfs", "create", "destroy", "punch", "sync", "get_info", "set_info"}
	hostname, _ = os.Hostname()
)

func check(e error) {
	if e != nil {
		log.Fatalf("ERROR: %v", e)
	}
}

func getMDTs() {
	files, err := ioutil.ReadDir(pathToMdts)
	check(err)
	for _, entry := range files {
		if entry.IsDir() {
			log.Println("Found:", entry.Name())
			mapMDTs[entry.Name()] = pathToMdts + "/" + entry.Name() + "/md_stats"
		}
	}
	if len(mapMDTs) == 0 {
		log.Fatal("No MTDs found.")
	}
}

func getOSTs() {
	files, err := ioutil.ReadDir(pathToOSTs)
	check(err)
	for _, entry := range files {
		if entry.IsDir() {
			log.Println("Found:", entry.Name())
			mapOSTs[entry.Name()] = pathToOSTs + "/" + entry.Name() + "/stats"
		}
	}
	if len(mapMDTs) == 0 {
		log.Fatal("No MTDs found.")
	}
}

func main() {

	var httpPort int

	flag.Uint64Var(&interval, "interval", 1, "Sample interval in seconds, the default is 1.")
	flag.IntVar(&httpPort, "port", 8666, "HTTP port used to access the the stats via web. Default is 8666.")

	flag.Parse()

	go func() {
		http.HandleFunc("/stats", httpStats)
		var baseURL = "localhost:" + strconv.Itoa(httpPort)
		log.Println(http.ListenAndServe(baseURL, nil))
	}()

	getMDTs()
	getOSTs()

	for {
		timeInterval := time.Duration(interval) * time.Second

		tm.Clear() // Clear current screen
		tm.MoveCursor(1, 1)
		currentTime := time.Now()
		strHeader := "Server: " + hostname + " | Time: " + currentTime.String() + " | Sample Interval: " + strconv.FormatUint(interval, 10) + "s"
		_, _ = tm.Println(tm.Background(tm.Color(tm.Bold(strHeader), tm.BLACK), tm.GREEN))

		var mapMDTPrevStats = make(map[string]map[string]uint64)
		var mapMDTNewStats = make(map[string]map[string]uint64)
		var mapMDTPrevStatsRaw = make(map[string][]byte)
		var mapMDTNewStatsRaw = make(map[string][]byte)
		var mapOSTPrevStats = make(map[string]map[string]uint64)
		var mapOSTNewStats = make(map[string]map[string]uint64)
		var mapOSTPrevStatsRaw = make(map[string][]byte)
		var mapOSTNewStatsRaw = make(map[string][]byte)

		for key, mdt := range mapMDTs {
			prevSample, err := ioutil.ReadFile(mdt)
			check(err)
			mapMDTPrevStatsRaw[key] = prevSample
		}
		for key, ost := range mapOSTs {
			prevSample, err := ioutil.ReadFile(ost)
			check(err)
			mapOSTPrevStatsRaw[key] = prevSample
		}

		time.Sleep(timeInterval)

		for key, mdt := range mapMDTs {
			newSample, err := ioutil.ReadFile(mdt)
			check(err)
			mapMDTNewStatsRaw[key] = newSample
		}

		for key, ost := range mapOSTs {
			newSample, err := ioutil.ReadFile(ost)
			check(err)
			mapOSTNewStatsRaw[key] = newSample
		}

		for mdt, value := range mapMDTPrevStatsRaw {
			var slcPrevStats = strings.Split(string(value), "\n")
			slcPrevStats = slcPrevStats[1 : len(slcPrevStats)-1]
			var prevCounters = make(map[string]uint64)

			for _, item := range slcPrevStats {
				var fields = strings.Fields(item)
				prevCounters[fields[0]], _ = strconv.ParseUint(fields[1], 10, 64)
				mapMDTPrevStats[mdt] = prevCounters
			}
		}
		for mdt, value := range mapMDTNewStatsRaw {
			var slcNewStats = strings.Split(string(value), "\n")
			slcNewStats = slcNewStats[1 : len(slcNewStats)-1]
			var newCounters = make(map[string]uint64)

			for _, item := range slcNewStats {
				var fields = strings.Fields(item)
				newCounters[fields[0]], _ = strconv.ParseUint(fields[1], 10, 64)
				mapMDTNewStats[mdt] = newCounters
			}
		}
		for ost, value := range mapOSTPrevStatsRaw {
			var slcPrevStats = strings.Split(string(value), "\n")
			slcPrevStats = slcPrevStats[1 : len(slcPrevStats)-1]
			var prevCounters = make(map[string]uint64)

			for _, item := range slcPrevStats {
				var fields = strings.Fields(item)
				if strings.Contains(fields[0], "bytes") {
					prevCounters[fields[0]], _ = strconv.ParseUint(fields[6], 10, 64)
				} else {
					prevCounters[fields[0]], _ = strconv.ParseUint(fields[1], 10, 64)
				}
				mapOSTPrevStats[ost] = prevCounters
			}
		}
		for ost, value := range mapOSTNewStatsRaw {
			var slcNewStats = strings.Split(string(value), "\n")
			slcNewStats = slcNewStats[1 : len(slcNewStats)-1]
			var newCounters = make(map[string]uint64)

			for _, item := range slcNewStats {
				var fields = strings.Fields(item)
				if strings.Contains(fields[0], "bytes") {
					newCounters[fields[0]], _ = strconv.ParseUint(fields[6], 10, 64)
				} else {
					newCounters[fields[0]], _ = strconv.ParseUint(fields[1], 10, 64)
				}
				mapOSTNewStats[ost] = newCounters
			}
		}
		for mdt, value := range mapMDTPrevStats {
			var mapCalcCounter = make(map[string]uint64)
			for key := range value {
				var calcCounter = (mapMDTNewStats[mdt][key] - mapMDTPrevStats[mdt][key]) / interval
				mapCalcCounter[key] = calcCounter
			}
			mapMDTCalcStats[mdt] = mapCalcCounter
		}
		for ost, value := range mapOSTPrevStats {
			var mapCalcCounter = make(map[string]uint64)
			for key := range value {
				var calcCounter = (mapOSTNewStats[ost][key] - mapOSTPrevStats[ost][key]) / interval
				mapCalcCounter[key] = calcCounter
			}
			mapOSTCalcStats[ost] = mapCalcCounter
		}
		tm.Flush()
		fmt.Println(tm.Bold("MDT Metadata Stats /s:"))
		fmt.Printf("%15s", "Device")
		for _, item := range mdtCounters {
			fmt.Printf("%10s", item)
		}
		fmt.Print("\n")
		for mdt, counters := range mapMDTCalcStats {
			fmt.Printf("%15s", mdt)
			for _, counter := range mdtCounters {
				if v, found := counters[counter]; found {
					fmt.Printf("%10d", v)
				} else {
					fmt.Printf("%10d", 0)
				}
			}
			fmt.Print("\n")
		}
		fmt.Println()
		fmt.Println(tm.Bold("OST Operation Stats /s:"))
		fmt.Printf("%15s", "Device")
		for _, item := range ostCounters {
			fmt.Printf("%13s", item)
		}
		fmt.Print("\n")
		for ost, counters := range mapOSTCalcStats {
			fmt.Printf("%15s", ost)
			for _, counter := range ostCounters {
				if v, found := counters[counter]; found {
					if strings.Contains(counter, "bytes") {
						fmt.Printf("%13s", humanize.Bytes(v))
					} else {
						fmt.Printf("%13d", v)
					}
				} else {
					fmt.Printf("%13d", 0)
				}
			}
			fmt.Print("\n")
		}
	}
}

func httpStats(w http.ResponseWriter, _ *http.Request) {
	currentTime := time.Now()
	strHeader := "Server: " + hostname + " | Time: " + currentTime.String() + " | Sample Interval: " + strconv.FormatUint(interval, 10) + "s"
	_, _ = fmt.Fprintln(w, strHeader)
	_, _ = fmt.Fprintln(w, "MDT Metadata Stats /s:")
	_, _ = fmt.Fprintf(w, "%15s", "Device")
	for _, item := range mdtCounters {
		_, _ = fmt.Fprintf(w, "%10s", item)
	}
	_, _ = fmt.Fprint(w, "\n")
	for mdt, counters := range mapMDTCalcStats {
		_, _ = fmt.Fprintf(w, "%15s", mdt)
		for _, counter := range mdtCounters {
			if v, found := counters[counter]; found {
				_, _ = fmt.Fprintf(w, "%10d", v)
			} else {
				_, _ = fmt.Fprintf(w, "%10d", 0)
			}
		}
		_, _ = fmt.Fprint(w, "\n")
	}
	_, _ = fmt.Fprintln(w, "OST Operation Stats /s:")
	_, _ = fmt.Fprintf(w, "%15s", "Device")
	for _, item := range ostCounters {
		_, _ = fmt.Fprintf(w, "%13s", item)
	}
	_, _ = fmt.Fprint(w, "\n")
	for ost, counters := range mapOSTCalcStats {
		_, _ = fmt.Fprintf(w, "%15s", ost)
		for _, counter := range ostCounters {
			if v, found := counters[counter]; found {
				if strings.Contains(counter, "bytes") {
					_, _ = fmt.Fprintf(w, "%13s", humanize.Bytes(v))
				} else {
					_, _ = fmt.Fprintf(w, "%13d", v)
				}
			} else {
				_, _ = fmt.Fprintf(w, "%13d", 0)
			}
		}
		_, _ = fmt.Fprint(w, "\n")
	}
}
