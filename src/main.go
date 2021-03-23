/*
MIT License

Copyright (c) 2020 storagebit.ch

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	tm "github.com/buger/goterm"
	"github.com/dustin/go-humanize"
	influxdb2 "github.com/influxdata/influxdb-client-go"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	interval int

	pathToMDTs = "/proc/fs/lustre/mdt"
	pathToOSTs = "/proc/fs/lustre/obdfilter"

	mapMDTCalcStats = make(map[string]map[string]uint64)
	mapOSTCalcStats = make(map[string]map[string]uint64)
	mapMDTJobStats  = make(map[string]map[string]map[string]uint64)
	mapOSTJobStats  = make(map[string]map[string]map[string]uint64)
	mapMDTs         = make(map[string]string)
	mapOSTs         = make(map[string]string)

	sortedMTDDevices []string
	sortedOSTDevices []string
	sortedMDTJobs    []string
	sortedOSTJobs    []string

	mdtCounters = []string{"open", "close", "mknod", "link", "unlink", "mkdir", "rmdir", "rename", "getattr",
		"setattr", "getxattr", "setxattr", "statfs", "sync", "read_bytes", "write_bytes"}
	mdtJobStatsCounters = []string{"open", "close", "mknod", "link", "unlink", "mkdir", "rmdir", "rename", "getattr",
		"setattr", "getxattr", "setxattr", "statfs", "sync", "read_bytes", "write_bytes"}
	ostCounters = []string{"write_bytes", "read_bytes", "setattr", "statfs", "create", "destroy", "punch", "sync",
		"get_info", "set_info"}
	ostJobStatsCounters = []string{"read_bytes", "write_bytes", "getattr", "setattr", "punch", "sync", "destroy",
		"create", "statfs", "get_info", "set_info", "quotactl"}

	hostname, _ = os.Hostname()

	ignoreMDTStats bool
	ignoreOSTStats bool
	reportJobStats bool
	runDaemonized  bool
	feedToInflux   bool
	influxServer   string
	influxPort     string
	influxOrg      string
	influxBucket   string
	influxToken    string
	flgVersion     bool
	buildSha1      string // sha1 revision used to build the program
	buildTime      string // when the executable was built
	buildBranch    string
)

func checkContinue(e error) {
	if e != nil {
		log.Printf("ERROR: %v", e)
	}
}

func getMDTs() {
	files, err := ioutil.ReadDir(pathToMDTs)
	checkContinue(err)
	for _, entry := range files {
		if entry.IsDir() {
			log.Println("Found:", entry.Name())
			mapMDTs[entry.Name()] = pathToMDTs + "/" + entry.Name() + "/md_stats"
		}
	}
	if len(mapMDTs) == 0 {
		log.Println("No MTDs found.")
		ignoreMDTStats = true
	}
}

func getOSTs() {
	files, err := ioutil.ReadDir(pathToOSTs)
	checkContinue(err)
	for _, entry := range files {
		if entry.IsDir() {
			log.Println("Found:", entry.Name())
			mapOSTs[entry.Name()] = pathToOSTs + "/" + entry.Name() + "/stats"
		}
	}
	if len(mapOSTs) == 0 {
		log.Println("No OSTs found.")
		ignoreOSTStats = true
	}
}

func readStatsFile(mapDevices map[string]string) map[string][]byte {

	var mapStatsRaw = make(map[string][]byte)

	for key, device := range mapDevices {
		rawStats, err := ioutil.ReadFile(device)
		if err != nil {
			log.Printf("ERROR: %v", err)
		} else {
			mapStatsRaw[key] = rawStats
		}
	}
	return mapStatsRaw
}

func readJobStatsFile(mapDevices map[string]string, deviceType string) map[string][]byte {

	var mapStatsRaw = make(map[string][]byte)

	for key := range mapDevices {
		rawStats, err := ioutil.ReadFile("/proc/fs/lustre/" + deviceType + "/" + key + "/job_stats")
		if err != nil {
			log.Printf("ERROR: %v", err)
		} else {
			mapStatsRaw[key] = rawStats
		}
	}
	return mapStatsRaw
}

func parseRAWSats(mapRAWStats map[string][]byte) map[string]map[string]uint64 {

	var mapStats = make(map[string]map[string]uint64)

	for device, value := range mapRAWStats {
		var slcStats = strings.Split(string(value), "\n")
		slcStats = slcStats[1 : len(slcStats)-1]
		var mapCounters = make(map[string]uint64)

		for _, item := range slcStats {
			var fields = strings.Fields(item)
			if strings.Contains(fields[0], "bytes") {
				mapCounters[fields[0]], _ = strconv.ParseUint(fields[6], 10, 64)
			} else {
				mapCounters[fields[0]], _ = strconv.ParseUint(fields[1], 10, 64)
			}
			mapStats[device] = mapCounters
		}
	}
	return mapStats
}

func calcStats(mapPrevStats map[string]map[string]uint64, mapNewStats map[string]map[string]uint64) map[string]map[string]uint64 {

	var mapStats = make(map[string]map[string]uint64)

	for device, value := range mapPrevStats {
		var mapCounter = make(map[string]uint64)
		for key := range value {
			var calcCounter = (mapNewStats[device][key] - mapPrevStats[device][key]) / uint64(interval)
			mapCounter[key] = calcCounter
		}
		mapStats[device] = mapCounter
	}
	return mapStats
}

func parseRAWJobStats(mapRAWJobStats map[string][]byte) map[string]map[string]map[string]uint64 {

	var mapJobStats = make(map[string]map[string]map[string]uint64)

	for device, value := range mapRAWJobStats {
		slcAllJobStats := strings.Split(string(value), "-")
		if len(value) <= 11 {
			continue
		} else {
			var mapStats = make(map[string]map[string]uint64)
			for _, item := range slcAllJobStats[1:] {
				slcJobStats := strings.Split(item, "\n")
				jobName := strings.Fields(slcJobStats[0])[1]
				var mapCounters = make(map[string]uint64)

				for _, line := range slcJobStats[2:] {
					if len(line) > 0 {
						var fields = strings.Fields(line)
						var counter = strings.TrimSuffix(strings.Fields(line)[0], ":")

						if strings.Contains(fields[0], "bytes") {
							mapCounters[counter], _ = strconv.ParseUint(fields[11], 10, 64)
						} else {
							var counterValue = strings.TrimSuffix(strings.Fields(line)[3], ",")
							mapCounters[counter], _ = strconv.ParseUint(counterValue, 10, 64)
						}
						mapStats[jobName] = mapCounters
					}
					mapJobStats[device] = mapStats
				}
			}
		}
	}
	return mapJobStats
}

func calcJobStats(mapPrevJobStats map[string]map[string]map[string]uint64, mapNewJobStats map[string]map[string]map[string]uint64) map[string]map[string]map[string]uint64 {

	var mapJobStats = make(map[string]map[string]map[string]uint64)

	for device, jobs := range mapPrevJobStats {
		var mapJobs = make(map[string]map[string]uint64)

		for job, counters := range jobs {
			var mapCounter = make(map[string]uint64)

			for key := range counters {
				var counter uint64
				counter = (mapNewJobStats[device][job][key] - mapPrevJobStats[device][job][key]) / uint64(interval)
				mapCounter[key] = counter
			}
			mapJobs[job] = mapCounter
		}
		mapJobStats[device] = mapJobs
	}
	return mapJobStats
}

func sortStatsMapIntoSlice(mapToSort map[string]map[string]uint64) []string {

	devices := make([]string, len(mapToSort))
	i := 0
	for device := range mapToSort {
		devices[i] = device
		i++
	}
	sort.Strings(devices)
	return devices
}

func sortJobsMapIntoSlice(mapToSort map[string]map[string]map[string]uint64) []string {
	var slcSortedDeviceJobs []string
	var slcDevices []string

	for device := range mapToSort {
		slcDevices = append(slcDevices, device)
	}
	sort.Strings(slcDevices)

	for _, device := range slcDevices {
		var slcJobs []string

		for job := range mapToSort[device] {
			slcJobs = append(slcJobs, job)
		}
		sort.Strings(slcJobs)

		for _, job := range slcJobs {
			slcSortedDeviceJobs = append(slcSortedDeviceJobs, device+"@@"+job)
		}
	}
	return slcSortedDeviceJobs
}

func printStats(mapStats map[string]map[string]uint64, slcDevices []string, slcCounters []string) {

	fmt.Printf("%20s", "Device")
	for _, item := range slcCounters {
		fmt.Printf("%13s", item)
	}
	fmt.Print("\n")
	for _, device := range slcDevices {
		fmt.Printf("%20s", device)
		for _, counter := range slcCounters {
			if v, found := mapStats[device][counter]; found {
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

func feedStatsToInflux(mapStats map[string]map[string]uint64, slcDevices []string, slcCounters []string) {

	influxClient := influxdb2.NewClient("http://"+influxServer+":"+influxPort, influxToken)
	var influxWriteAPI = influxClient.WriteAPI(influxOrg, influxBucket)

	for _, device := range slcDevices {
		influxLine := "lure,server=" + hostname + ",device=" + device + ",type=stats "
		var fieldKeyValues []string
		for _, counter := range slcCounters {
			if v, found := mapStats[device][counter]; found {
				fieldKeyValues = append(fieldKeyValues, counter+"="+strconv.FormatUint(v, 10))
			}
		}
		influxWriteAPI.WriteRecord(fmt.Sprintf(influxLine + " " + strings.Join(fieldKeyValues, ",")))
	}
	influxWriteAPI.Flush()
	influxClient.Close()
}

func printJobStats(mapJobStats map[string]map[string]map[string]uint64, slcJobs []string, slcCounters []string) {

	fmt.Printf("%20s", "Job @ Device")

	for _, item := range slcCounters {
		fmt.Printf("%13s", item)
	}
	fmt.Print("\n")

	for _, jobHash := range slcJobs {
		var device = strings.Split(jobHash, "@@")[0]
		var job = strings.Split(jobHash, "@@")[1]
		fmt.Printf("%20s", job+"@"+strings.Split(device, "-")[1])
		for _, counter := range slcCounters {
			if v, found := mapJobStats[device][job][counter]; found {
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

func feedJobStatsToInflux(mapJobStats map[string]map[string]map[string]uint64, slcJobs []string, slcCounters []string) {

	influxClient := influxdb2.NewClient("http://"+influxServer+":"+influxPort, influxToken)
	var influxWriteAPI = influxClient.WriteAPI(influxOrg, influxBucket)

	for _, jobHash := range slcJobs {
		var device = strings.Split(jobHash, "@@")[0]
		var job = strings.Split(jobHash, "@@")[1]

		influxLine := "lure,server=" + hostname + ",device=" + device + ",type=job_stats," + "job=" + job + " "
		var fieldKeyValues []string
		for _, counter := range slcCounters {
			if v, found := mapJobStats[device][job][counter]; found {
				fieldKeyValues = append(fieldKeyValues, counter+"="+strconv.FormatUint(v, 10))
			}
		}
		influxWriteAPI.WriteRecord(fmt.Sprintf(influxLine + " " + strings.Join(fieldKeyValues, ",")))
	}
	influxWriteAPI.Flush()
	influxClient.Close()
}

func main() {

	var httpPort int

	flag.IntVar(&interval, "interval", 1, "Sample interval in seconds")
	flag.IntVar(&httpPort, "port", 8666, "HTTP port used to access the the stats via web browser.")
	flag.BoolVar(&ignoreMDTStats, "ignoremdt", false, "Don't report MDT stats.")
	flag.BoolVar(&ignoreOSTStats, "ignore", false, "Don't report OST stats.")
	flag.BoolVar(&reportJobStats, "jobstats", false, "Report Lustre Jobstats for MDT and OST devices.")
	flag.BoolVar(&runDaemonized, "daemon", false, "Run as daemon in the background. No console output but stats available via web interface.")
	flag.BoolVar(&flgVersion, "version", false, "Print version information.")
	flag.BoolVar(&feedToInflux, "feedtoinflux", false, "Store statistics in InfluxDB")
	flag.StringVar(&influxServer, "influxserver", "localhost", "InfluxDB server name or IP")
	flag.StringVar(&influxPort, "influxport", "8086", "InfluxDB server port")
	flag.StringVar(&influxOrg, "influxorg", "storagebit", "InfluxDB org")
	flag.StringVar(&influxBucket, "influxbucket", "lure", "InfluxDB bucket")
	flag.StringVar(&influxToken, "influxtoken",
		"lure:password",
		"Read/Write token for the bucket or user:password in the InfluxDB")

	flag.Parse()

	if flgVersion {
		fmt.Printf("Build on: %s from branch: %s with sha1: %s\n", buildTime, buildBranch, buildSha1)
		os.Exit(0)
	}

	go func() {
		http.HandleFunc("/stats", httpStats)
		http.HandleFunc("/json", jsonStats)
		var baseURL = "localhost:" + strconv.Itoa(httpPort)
		err := http.ListenAndServe(baseURL, nil)
		checkContinue(err)
	}()

	if ignoreMDTStats != true {
		getMDTs()
	}
	if ignoreOSTStats != true {
		getOSTs()
	}

	for {
		timeInterval := time.Duration(interval) * time.Second

		if runDaemonized != true {
			tm.Clear()
			tm.MoveCursor(1, 1)
			currentTime := time.Now()
			strHeader := "Server: " + hostname + " | Time: " + currentTime.String() + " | Sample Interval: " +
				strconv.Itoa(interval) + "s"
			_, _ = tm.Println(tm.Background(tm.Color(tm.Bold(strHeader), tm.BLACK), tm.GREEN))
		}

		var mapMDTPrevStats = make(map[string]map[string]uint64)
		var mapMDTNewStats = make(map[string]map[string]uint64)
		var mapMDTPrevStatsRaw = make(map[string][]byte)
		var mapMDTNewStatsRaw = make(map[string][]byte)
		var mapMDTNewJobStatsRaw = make(map[string][]byte)
		var mapOSTPrevStats = make(map[string]map[string]uint64)
		var mapOSTNewStats = make(map[string]map[string]uint64)
		var mapOSTPrevStatsRaw = make(map[string][]byte)
		var mapOSTNewStatsRaw = make(map[string][]byte)
		var mapMDTPrevJobStats = make(map[string]map[string]map[string]uint64)
		var mapMDTNewJobStats = make(map[string]map[string]map[string]uint64)
		var mapMDTPrevJobStatsRaw = make(map[string][]byte)
		var mapOSTNewJobStats = make(map[string]map[string]map[string]uint64)
		var mapOSTPrevJobStats = make(map[string]map[string]map[string]uint64)
		var mapOSTPrevJobStatsRaw = make(map[string][]byte)
		var mapOSTNewJobStatsRaw = make(map[string][]byte)

		if ignoreMDTStats != true {
			mapMDTPrevStatsRaw = readStatsFile(mapMDTs)
		}
		if ignoreOSTStats != true {
			mapOSTPrevStatsRaw = readStatsFile(mapOSTs)
		}
		if reportJobStats == true {
			mapMDTPrevJobStatsRaw = readJobStatsFile(mapMDTs, "mdt")
			mapOSTPrevJobStatsRaw = readJobStatsFile(mapOSTs, "obdfilter")
		}

		time.Sleep(timeInterval)

		if ignoreMDTStats != true {
			mapMDTNewStatsRaw = readStatsFile(mapMDTs)
		}
		if ignoreOSTStats != true {
			mapOSTNewStatsRaw = readStatsFile(mapOSTs)
		}
		if reportJobStats == true {
			mapMDTNewJobStatsRaw = readJobStatsFile(mapMDTs, "mdt")
			mapOSTNewJobStatsRaw = readJobStatsFile(mapOSTs, "obdfilter")
		}

		if ignoreMDTStats != true {
			mapMDTPrevStats = parseRAWSats(mapMDTPrevStatsRaw)
			mapMDTNewStats = parseRAWSats(mapMDTNewStatsRaw)
			mapMDTCalcStats = calcStats(mapMDTPrevStats, mapMDTNewStats)
		}
		if ignoreOSTStats != true {
			mapOSTPrevStats = parseRAWSats(mapOSTPrevStatsRaw)
			mapOSTNewStats = parseRAWSats(mapOSTNewStatsRaw)
			mapOSTCalcStats = calcStats(mapOSTPrevStats, mapOSTNewStats)
		}

		if reportJobStats == true {
			mapMDTPrevJobStats = parseRAWJobStats(mapMDTPrevJobStatsRaw)
			mapMDTNewJobStats = parseRAWJobStats(mapMDTNewJobStatsRaw)
			mapOSTPrevJobStats = parseRAWJobStats(mapOSTPrevJobStatsRaw)
			mapOSTNewJobStats = parseRAWJobStats(mapOSTNewJobStatsRaw)
			mapMDTJobStats = calcJobStats(mapMDTPrevJobStats, mapMDTNewJobStats)
			mapOSTJobStats = calcJobStats(mapOSTPrevJobStats, mapOSTNewJobStats)
			sortedMDTJobs = sortJobsMapIntoSlice(mapMDTJobStats)
			sortedOSTJobs = sortJobsMapIntoSlice(mapOSTJobStats)
		}

		sortedMTDDevices = sortStatsMapIntoSlice(mapMDTCalcStats)
		sortedOSTDevices = sortStatsMapIntoSlice(mapOSTCalcStats)

		if runDaemonized != true {

			tm.Flush()
			fmt.Println(tm.Bold("MDT Metadata Stats /s:"))
			if len(mapMDTCalcStats) != 0 {
				printStats(mapMDTCalcStats, sortedMTDDevices, mdtCounters)
				if feedToInflux {
					feedStatsToInflux(mapMDTCalcStats, sortedMTDDevices, mdtCounters)
				}
			} else {
				fmt.Println("No MDT stats available.")
			}
			fmt.Println()
			fmt.Println(tm.Bold("OST Operation Stats /s:"))
			if len(mapOSTCalcStats) != 0 {
				printStats(mapOSTCalcStats, sortedOSTDevices, ostCounters)
				if feedToInflux {
					feedStatsToInflux(mapOSTCalcStats, sortedOSTDevices, ostCounters)
				}
			} else {
				fmt.Println("No OST stats available.")
			}
			fmt.Println()
			fmt.Println(tm.Bold("MDT Jobstats /s:"))
			if len(mapMDTJobStats) != 0 {
				printJobStats(mapMDTJobStats, sortedMDTJobs, mdtJobStatsCounters)
				if feedToInflux {
					feedJobStatsToInflux(mapMDTJobStats, sortedMDTJobs, mdtJobStatsCounters)
				}
			} else {
				fmt.Println("No MDT Jobstats available.")
			}
			fmt.Println()
			fmt.Println(tm.Bold("OST Jobstats /s:"))
			if len(mapOSTJobStats) != 0 {
				printJobStats(mapOSTJobStats, sortedOSTJobs, ostJobStatsCounters)
				if feedToInflux {
					feedJobStatsToInflux(mapOSTJobStats, sortedOSTJobs, ostJobStatsCounters)
				}
			} else {
				fmt.Println("No OST Jobstats available.")
			}
		} else {
			if feedToInflux {
				if len(mapMDTCalcStats) != 0 {
					feedStatsToInflux(mapMDTCalcStats, sortedMTDDevices, mdtCounters)
				}
				if len(mapOSTCalcStats) != 0 {
					feedStatsToInflux(mapOSTCalcStats, sortedOSTDevices, ostCounters)
				}
				if len(mapMDTJobStats) != 0 {
					feedJobStatsToInflux(mapMDTJobStats, sortedMDTJobs, mdtJobStatsCounters)
				}
				if len(mapOSTJobStats) != 0 {
					feedJobStatsToInflux(mapOSTJobStats, sortedOSTJobs, ostJobStatsCounters)
				}
			}
		}
	}
}

func httpStats(w http.ResponseWriter, _ *http.Request) {
	currentTime := time.Now()
	strHeader := "Server: " + hostname + " | Time: " + currentTime.String() + " | Sample Interval: " +
		strconv.Itoa(interval) + "s"
	_, _ = fmt.Fprintln(w, strHeader)
	_, _ = fmt.Fprintln(w, "MDT Metadata Stats /s:")
	_, _ = fmt.Fprintf(w, "%15s", "Device")
	for _, item := range mdtCounters {
		_, _ = fmt.Fprintf(w, "%10s", item)
	}
	_, _ = fmt.Fprint(w, "\n")
	for mdt, counters := range mapMDTCalcStats {
		_, _ = fmt.Fprintf(w, "%20s", mdt)
		for _, counter := range mdtCounters {
			if v, found := counters[counter]; found {
				_, _ = fmt.Fprintf(w, "%13d", v)
			} else {
				_, _ = fmt.Fprintf(w, "%13d", 0)
			}
		}
		_, _ = fmt.Fprint(w, "\n")
	}
	_, _ = fmt.Fprintln(w, "\nOST Operation Stats /s:")
	_, _ = fmt.Fprintf(w, "%20s", "Device")
	for _, item := range ostCounters {
		_, _ = fmt.Fprintf(w, "%13s", item)
	}
	_, _ = fmt.Fprint(w, "\n")
	for ost, counters := range mapOSTCalcStats {
		_, _ = fmt.Fprintf(w, "%20s", ost)
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
	_, _ = fmt.Fprint(w, "\n")

	_, _ = fmt.Fprint(w, "MDT Jobstats /s:")
	_, _ = fmt.Fprint(w, "\n")
	if len(mapMDTJobStats) != 0 {
		_, _ = fmt.Fprintf(w, "%20s", "Job @ Device")
		for _, item := range mdtJobStatsCounters {
			_, _ = fmt.Fprintf(w, "%13s", item)
		}
		_, _ = fmt.Fprint(w, "\n")
		for mdt, jobs := range mapMDTJobStats {
			for job, counters := range jobs {
				_, _ = fmt.Fprintf(w, "%20s", job+"@"+strings.Split(mdt, "-")[1])
				for _, counter := range mdtJobStatsCounters {
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
	} else {
		_, _ = fmt.Fprint(w, "\nNo MDT Jobstats available.")
	}
	if len(mapOSTJobStats) != 0 {
		_, _ = fmt.Fprintf(w, "%20s", "Job @ Device")
		for _, item := range ostJobStatsCounters {
			_, _ = fmt.Fprintf(w, "%13s", item)
		}
		_, _ = fmt.Fprint(w, "\n")
		for ost, jobs := range mapOSTJobStats {
			for job, counters := range jobs {
				_, _ = fmt.Fprintf(w, "%20s", job+"@"+strings.Split(ost, "-")[1])
				for _, counter := range mdtJobStatsCounters {
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
	} else {
		_, _ = fmt.Fprint(w, "\nNo OST Jobstats available.")
	}
}

func jsonStats(w http.ResponseWriter, r *http.Request) {

	keys := r.URL.Query()
	urlRequest := keys.Get("stats") //Get return empty string if key not found

	if len(urlRequest) > 0 {
		w.Header().Set("Content-Type", "application/json")
		switch urlRequest {
		case "mdt":
			if len(mapMDTCalcStats) > 0 {
				jsonData, _ := json.Marshal(mapMDTCalcStats)
				_, _ = w.Write(jsonData)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		case "ost":
			if len(mapOSTCalcStats) > 0 {
				jsonData, _ := json.Marshal(mapOSTCalcStats)
				_, _ = w.Write(jsonData)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		case "mdtjob":
			if len(mapMDTJobStats) > 0 {
				jsonData, _ := json.Marshal(mapMDTJobStats)
				_, _ = w.Write(jsonData)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		case "ostjob":
			if len(mapOSTJobStats) > 0 {
				jsonData, _ := json.Marshal(mapOSTJobStats)
				_, _ = w.Write(jsonData)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
}
