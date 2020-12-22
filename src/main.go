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
	interval int

	pathToMDTs = "/proc/fs/lustre/mdt"
	pathToOSTs = "/proc/fs/lustre/obdfilter"

	mapMDTCalcStats = make(map[string]map[string]uint64)
	mapOSTCalcStats = make(map[string]map[string]uint64)
	mapMDTJobStats  = make(map[string]map[string]map[string]uint64)
	mapOSTJobStats  = make(map[string]map[string]map[string]uint64)
	mapMDTs         = make(map[string]string)
	mapOSTs         = make(map[string]string)

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

func main() {

	var httpPort int

	flag.IntVar(&interval, "interval", 1, "Sample interval in seconds")
	flag.IntVar(&httpPort, "port", 8666, "HTTP port used to access the the stats via web browser.")
	flag.BoolVar(&ignoreMDTStats, "ignoremdt", false, "Don't report MDT stats.")
	flag.BoolVar(&ignoreOSTStats, "ignoreost", false, "Don't report OST stats.")
	flag.BoolVar(&reportJobStats, "jobstats", false, "Report Lustre Jobstats for MDT and OST devices.")
	flag.BoolVar(&runDaemonized, "daemon", false, "Run as daemon in the background. No console output but stats available via web interface.")
	flag.BoolVar(&flgVersion, "version", false, "Print version information.")

	flag.Parse()

	if flgVersion {
		fmt.Printf("Build on: %s from branch: %s with sha1: %s\n", buildTime, buildBranch, buildSha1)
		os.Exit(0)
	}

	go func() {
		http.HandleFunc("/stats", httpStats)
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
			for key, mdt := range mapMDTs {
				prevSample, err := ioutil.ReadFile(mdt)
				if err != nil {
					log.Printf("ERROR: %v", err)
				} else {
					mapMDTPrevStatsRaw[key] = prevSample
				}
			}
		}
		if ignoreOSTStats != true {
			for key, ost := range mapOSTs {
				prevSample, err := ioutil.ReadFile(ost)
				if err != nil {
					log.Printf("ERROR: %v", err)
				} else {
					mapOSTPrevStatsRaw[key] = prevSample
				}
			}
		}
		if reportJobStats == true {
			for mdt := range mapMDTs {
				prevSample, err := ioutil.ReadFile("/proc/fs/lustre/mdt/" + mdt + "/job_stats")
				if err != nil {
					log.Printf("ERROR: %v", err)
				} else {
					mapMDTPrevJobStatsRaw[mdt] = prevSample
				}
			}
			for ost := range mapOSTs {
				prevSample, err := ioutil.ReadFile("/proc/fs/lustre/obdfilter/" + ost + "/job_stats")
				if err != nil {
					log.Printf("ERROR: %v", err)
				} else {
					mapOSTPrevJobStatsRaw[ost] = prevSample
				}
			}
		}

		time.Sleep(timeInterval)

		if ignoreMDTStats != true {
			for key, mdt := range mapMDTs {
				newSample, err := ioutil.ReadFile(mdt)
				if err != nil {
					log.Printf("ERROR: %v", err)
				} else {
					mapMDTNewStatsRaw[key] = newSample
				}
			}
		}
		if ignoreOSTStats != true {
			for key, ost := range mapOSTs {
				newSample, err := ioutil.ReadFile(ost)
				if err != nil {
					log.Printf("ERROR: %v", err)
				} else {
					mapOSTNewStatsRaw[key] = newSample
				}
			}
		}
		if reportJobStats == true {
			for mdt := range mapMDTs {
				newSample, err := ioutil.ReadFile("/proc/fs/lustre/mdt/" + mdt + "/job_stats")
				if err != nil {
					log.Printf("ERROR: %v", err)
				} else {
					mapMDTNewJobStatsRaw[mdt] = newSample
				}
			}
			for ost := range mapOSTs {
				newSample, err := ioutil.ReadFile("/proc/fs/lustre/obdfilter/" + ost + "/job_stats")
				if err != nil {
					log.Printf("ERROR: %v", err)
				} else {
					mapOSTNewJobStatsRaw[ost] = newSample
				}
			}
		}
		if ignoreMDTStats != true {
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
			for mdt, value := range mapMDTPrevStats {
				var mapCalcCounter = make(map[string]uint64)
				for key := range value {
					var calcCounter = (mapMDTNewStats[mdt][key] - mapMDTPrevStats[mdt][key]) / uint64(interval)
					mapCalcCounter[key] = calcCounter
				}
				mapMDTCalcStats[mdt] = mapCalcCounter
			}
		}
		if ignoreOSTStats != true {
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
			for ost, value := range mapOSTPrevStats {
				var mapCalcCounter = make(map[string]uint64)
				for key := range value {
					var calcCounter = (mapOSTNewStats[ost][key] - mapOSTPrevStats[ost][key]) / uint64(interval)
					mapCalcCounter[key] = calcCounter
				}
				mapOSTCalcStats[ost] = mapCalcCounter
			}
		}
		if reportJobStats == true {

			// parsing MDT raw jobstats
			for mdt, value := range mapMDTPrevJobStatsRaw {
				slcAllJobStats := strings.Split(string(value), "-")

				if len(value) <= 11 {
					continue
				} else {
					var mapPrevJobStats = make(map[string]map[string]uint64)
					for _, item := range slcAllJobStats[1:] {
						slcJobStats := strings.Split(item, "\n")
						jobName := strings.Fields(slcJobStats[0])[1]
						var prevCounters = make(map[string]uint64)

						for _, line := range slcJobStats[2:] {
							if len(line) > 0 {
								var fields = strings.Fields(line)
								var counter = strings.TrimSuffix(strings.Fields(line)[0], ":")

								if strings.Contains(fields[0], "bytes") {
									prevCounters[counter], _ = strconv.ParseUint(fields[11], 10, 64)
								} else {
									var counterValue = strings.TrimSuffix(strings.Fields(line)[3], ",")
									prevCounters[counter], _ = strconv.ParseUint(counterValue, 10, 64)
								}
								mapPrevJobStats[jobName] = prevCounters
							}
							mapMDTPrevJobStats[mdt] = mapPrevJobStats
						}
					}
				}
			}
			for mdt, value := range mapMDTNewJobStatsRaw {
				slcAllJobStats := strings.Split(string(value), "-")
				if len(value) <= 11 {
					continue
				} else {
					var mapNewJobStats = make(map[string]map[string]uint64)
					for _, item := range slcAllJobStats[1:] {
						slcJobStats := strings.Split(item, "\n")
						jobName := strings.Fields(slcJobStats[0])[1]
						var newCounter = make(map[string]uint64)

						for _, line := range slcJobStats[2:] {
							if len(line) > 0 {
								var fields = strings.Fields(line)
								var counter = strings.TrimSuffix(strings.Fields(line)[0], ":")
								if strings.Contains(fields[0], "bytes") {
									newCounter[counter], _ = strconv.ParseUint(fields[11], 10, 64)
								} else {
									var counterValue = strings.TrimSuffix(strings.Fields(line)[3], ",")
									newCounter[counter], _ = strconv.ParseUint(counterValue, 10, 64)
								}
								mapNewJobStats[jobName] = newCounter
							}
							mapMDTNewJobStats[mdt] = mapNewJobStats
						}
					}
				}
			}

			// parsing OST raw jobstats
			for ost, value := range mapOSTPrevJobStatsRaw {
				slcAllJobStats := strings.Split(string(value), "-")

				if len(value) <= 11 {
					continue
				} else {
					var mapPrevJobStats = make(map[string]map[string]uint64)
					for _, item := range slcAllJobStats[1:] {
						slcJobStats := strings.Split(item, "\n")
						jobName := strings.Fields(slcJobStats[0])[1]
						var prevCounters = make(map[string]uint64)

						for _, line := range slcJobStats[2:] {
							if len(line) > 0 {
								var fields = strings.Fields(line)
								var counter = strings.TrimSuffix(strings.Fields(line)[0], ":")

								if strings.Contains(counter, "bytes") {
									prevCounters[counter], _ = strconv.ParseUint(fields[11], 10, 64)
								} else {
									var counterValue = strings.TrimSuffix(strings.Fields(line)[3], ",")
									prevCounters[counter], _ = strconv.ParseUint(counterValue, 10, 64)
								}
								mapPrevJobStats[jobName] = prevCounters
							}
							mapOSTPrevJobStats[ost] = mapPrevJobStats
						}
					}
				}
			}
			for ost, value := range mapOSTNewJobStatsRaw {
				slcAllJobStats := strings.Split(string(value), "-")
				if len(value) <= 11 {
					continue
				} else {
					var mapNewJobStats = make(map[string]map[string]uint64)
					for _, item := range slcAllJobStats[1:] {
						slcJobStats := strings.Split(item, "\n")
						jobName := strings.Fields(slcJobStats[0])[1]
						var newCounter = make(map[string]uint64)

						for _, line := range slcJobStats[2:] {
							if len(line) > 0 {
								var fields = strings.Fields(line)
								var counter = strings.TrimSuffix(strings.Fields(line)[0], ":")
								if strings.Contains(counter, "bytes") {
									newCounter[counter], _ = strconv.ParseUint(fields[11], 10, 64)
								} else {
									var counterValue = strings.TrimSuffix(strings.Fields(line)[3], ",")
									newCounter[counter], _ = strconv.ParseUint(counterValue, 10, 64)
								}
								mapNewJobStats[jobName] = newCounter
							}
							mapOSTNewJobStats[ost] = mapNewJobStats
						}
					}
				}
			}
		}

		// calculating the MDT jobstats
		for mdt, jobs := range mapMDTNewJobStats {
			var mapMDTJobs = make(map[string]map[string]uint64)
			for job, counters := range jobs {
				var mapCalcCounter = make(map[string]uint64)

				for key := range counters {
					var prevCounter = mapMDTPrevJobStats[mdt][job][key]
					var newCounter = mapMDTNewJobStats[mdt][job][key]
					var calcCounter uint64
					calcCounter = (newCounter - prevCounter) / uint64(interval)
					mapCalcCounter[key] = calcCounter
				}
				mapMDTJobs[job] = mapCalcCounter
			}
			mapMDTJobStats[mdt] = mapMDTJobs
		}

		// calculating the OST jobstats
		for ost, jobs := range mapOSTNewJobStats {
			var mapOSTJobs = make(map[string]map[string]uint64)
			for job, counters := range jobs {
				var mapCalcCounter = make(map[string]uint64)

				for key := range counters {
					var prevCounter = mapOSTPrevJobStats[ost][job][key]
					var newCounter = mapOSTNewJobStats[ost][job][key]
					var calcCounter uint64
					calcCounter = (newCounter - prevCounter) / uint64(interval)
					mapCalcCounter[key] = calcCounter
				}
				mapOSTJobs[job] = mapCalcCounter
			}
			mapOSTJobStats[ost] = mapOSTJobs
		}

		if runDaemonized != true {
			tm.Flush()
			fmt.Println(tm.Bold("MDT Metadata Stats /s:"))
			if len(mapMDTCalcStats) != 0 {
				fmt.Printf("%20s", "Device")
				for _, item := range mdtCounters {
					fmt.Printf("%13s", item)
				}
				fmt.Print("\n")
				for mdt, counters := range mapMDTCalcStats {
					fmt.Printf("%20s", mdt)
					for _, counter := range mdtCounters {
						if v, found := counters[counter]; found {
							fmt.Printf("%13d", v)
						} else {
							fmt.Printf("%13d", 0)
						}
					}
					fmt.Print("\n")
				}
			} else {
				fmt.Println("No MDT stats available.")
			}
			fmt.Println()
			fmt.Println(tm.Bold("OST Operation Stats /s:"))
			if len(mapOSTCalcStats) != 0 {
				fmt.Printf("%20s", "Device")
				for _, item := range ostCounters {
					fmt.Printf("%13s", item)
				}
				fmt.Print("\n")
				for ost, counters := range mapOSTCalcStats {
					fmt.Printf("%20s", ost)
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
			} else {
				fmt.Println("No OST stats available.")
			}
			fmt.Println()
			fmt.Println(tm.Bold("MDT Jobstats /s:"))
			if len(mapMDTJobStats) != 0 {
				fmt.Printf("%20s", "Job @ Device")
				for _, item := range mdtJobStatsCounters {
					fmt.Printf("%13s", item)
				}
				fmt.Print("\n")

				for mdt, jobs := range mapMDTJobStats {
					for job, counters := range jobs {
						fmt.Printf("%20s", job+"@"+strings.Split(mdt, "-")[1])
						for _, counter := range mdtJobStatsCounters {
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
			} else {
				fmt.Println("No MDT Jobstats available.")
			}
			fmt.Println()
			fmt.Println(tm.Bold("OST Jobstats /s:"))
			if len(mapOSTJobStats) != 0 {
				fmt.Printf("%20s", "Job @ Device")
				for _, item := range ostJobStatsCounters {
					fmt.Printf("%13s", item)
				}
				fmt.Print("\n")

				for ost, jobs := range mapOSTJobStats {
					for job, counters := range jobs {
						fmt.Printf("%20s", job+"@"+strings.Split(ost, "-")[1])
						for _, counter := range ostJobStatsCounters {
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
			} else {
				fmt.Println("No OST Jobstats available.")
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
