package main

import (
	"bufio"
	"flag"
	"fmt"
	_ "net"
	"os"
	"strings"
	"time"

	"math/rand"
	"net/http"
)

func setReqHeader(req *http.Request) {
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept-Language", "en-us")
	req.Header.Set("Aceept", "*/*")
}

func send(client *http.Client, method, url, data string, setHeader func(req *http.Request)) *http.Response {
	body := strings.NewReader(data)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(fmt.Sprint("http new request error:", err))
	}
	setHeader(req)
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("client do error:", err)
		return nil
	}
	return res
}

type link_state struct {
	start int64
	end   int64
	state int
	url   string
}

var method string
var latency chan float64
var latency2 chan link_state
var timeout int
var fifo0 = make(chan string)
var die = make(chan bool)

func consume() {
	timeoutDuration := time.Duration(time.Duration(timeout) * time.Second)
	client := &http.Client{
		Timeout: timeoutDuration,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	for url := range fifo0 {
		start := time.Now()
		res := send(client, method, url, "", setReqHeader)
		end := time.Now()
		if res == nil {
			latency2 <- link_state{start.UnixNano(), end.UnixNano(), 404, url}
		}
		if res != nil {
			latency2 <- link_state{start.UnixNano(), end.UnixNano(), res.StatusCode, url}
		}
		if res == nil {
			fmt.Printf("HTTP   fail:     %s %s\n", method, url)
			continue
		} else {
			res.Body.Close()
		}
		delta := end.Sub(start).Seconds()
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			latency <- delta
		}
		fmt.Printf("HTTP %d   %.2f secs:     %s %s\n", res.StatusCode, delta, method, url)
	}
	die <- true
}

func main() {
	rand.Seed(time.Now().UnixNano())

	var ifile string
	flag.StringVar(&ifile, "i", "-", "read from url.list")

	flag.StringVar(&method, "X", "POST", "request method")

	var conNum int
	flag.IntVar(&conNum, "C", 1, "the concurrency number")

	var total int
	flag.IntVar(&total, "n", 1, "the number of requests")

	flag.IntVar(&timeout, "t", 1, "timeout")

    var savfile, savefile1, savefile2 string
    flag.StringVar(&savfile, "s", "", "set save file path")

	var notSaveLatency bool
	flag.BoolVar(&notSaveLatency, "nosave", false, "do not save latency statistics")

	flag.Parse()

    if (savfile != "") {
        savefile1 = "/tmp/latency-" + savfile + ".txt"
        savefile2 = "/tmp/latency2-" + savfile + ".txt"
    } else {
        savefile1 = "/tmp/latency.txt"
        savefile2 = "/tmp/latency2.txt"
    }
	var latencyFile *os.File
	var latencyFile2 *os.File
	if (!notSaveLatency) {
		var err error
		latencyFile, err = os.Create(savefile1)
		if err != nil {
			fmt.Println("latency.txt file create failed:", err)
			os.Exit(1)
		}
		latencyFile2, err = os.Create(savefile2)
		if err != nil {
			fmt.Println("latency2.txt file create failed:", err)
			os.Exit(1)
		}
	}

	var reader *os.File
	if ifile == "-" {
		reader = os.Stdin
	} else {
		var err error
		reader, err = os.Open(ifile)
		if err != nil {
			fmt.Println("open file error:", err)
			os.Exit(1)
		}
	}
	scanner := bufio.NewScanner(reader)

	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("reading error:", err)
	}

	latency = make(chan float64, total)
	latency2 = make(chan link_state, total)

	sstart := time.Now()
	for i := 0; i < conNum; i++ {
		go consume()
	}
	for i := 0; i < total; i++ {
		num := rand.Intn(len(lines))
		fifo0 <- lines[num]
	}
	close(fifo0)
	for i := 0; i < conNum; i++ {
		<-die
	}
	eend := time.Now()

	close(latency)
	close(latency2)
	successReq := 0
	max := 0.0
	min := 1000000.0
	avg := 0.0
	for d := range latency {
		if max < d {
			max = d
		}
		if min > d {
			min = d
		}
		avg += d
		successReq++
		if (!notSaveLatency) {
			latencyFile.Write([]byte(fmt.Sprintf("%.2f\n", d)))
		}
	}
    // Drag data from latency2
    latency2_ := make([]link_state, 0)
    for d:= range latency2 {
        latency2_ = append(latency2_, d)
    }
    cnt_interval := 0.1 // 0.1s
    interval := int64(cnt_interval * 1000000000)
    start_time := time.Now().UnixNano()
    end_time := int64(0)
    for _, d:= range latency2_ {
        start := d.start
        end := d.end
        if (start_time > start) {
            start_time = start
        }
        if (end_time < end) {
            end_time = end
        }
    }
    end_interval := int((end_time - start_time) / interval)
    throughput := make([]int, end_interval + 1)
    for _, d:= range latency2_ {
        state := d.state
        end := d.end
        if (state >= 200 && state < 300) {
            throughput[(end - start_time) / interval] += 1
            fmt.Printf("interval: %d, cnt: %d\n", (end - start_time) / interval, throughput[(end - start_time) / interval])
        }
    }
    max_throughput := float64(0)
    avg_throughput := float64(0)
    interval_cnt := 0
	for i := 0; i < end_interval; i++ {
        if ((float64(throughput[i]) / cnt_interval) > max_throughput) {
            max_throughput = float64(throughput[i]) / cnt_interval
        }
        if (i >= (end_interval / 10) && i < (end_interval / 10 * 9)) {
            avg_throughput += float64(throughput[i]) / cnt_interval
            interval_cnt += 1
        }
    }
	if (!notSaveLatency) {
		for _, d := range latency2_ {
			start := d.start
			end := d.end
			state := d.state
			url := d.url
			latencyFile2.Write([]byte(fmt.Sprintf("%d %d %d\n", start, end, state)))
			latencyFile2.Write([]byte(fmt.Sprintf("%s\n", url)))
		}
	}

	avg /= float64(successReq)

	// calc Variance
	variance := 0.0
	for d := range latency {
		variance += (avg - d) * (avg - d)
	}
	variance /= float64(successReq);

	totalReq := total
	fmt.Println(os.Args)
	fmt.Printf("Trasnaction:                %d hits\n", totalReq)
	fmt.Printf("Availability:               %.2f %%\n", 100*float64(successReq)/float64(totalReq))
	fmt.Printf("Elapsed time:               %.2f secs\n", eend.Sub(sstart).Seconds())
	//fmt.Println("Data transferred:            ")
	fmt.Printf("Transcation rate:           %.2f trans/sec\n", float64(totalReq)/eend.Sub(sstart).Seconds())
	fmt.Printf("Successful trasnaction:     %d\n", successReq)
	fmt.Printf("Failed trasnaction:         %d\n", totalReq-successReq)
	fmt.Printf("Longest transaction:        %.2f\n", max)
	fmt.Printf("Shortest transaction:       %.2f\n", min)
	fmt.Printf("Average transaction:        %.2f\n", avg)
	fmt.Printf("Transaction Variance:       %.2f\n", variance)
	fmt.Printf("Estimate Throughput:        %.2f\n", 1.0/avg*float64(conNum))
	fmt.Printf("Max Throughput:        %.2f\n", float64(max_throughput))
	fmt.Printf("Avg Throughput:        %.2f\n", float64(avg_throughput) / float64(interval_cnt))
}
