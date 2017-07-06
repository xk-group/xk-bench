package main

import (
	_"net"
	"bufio"
	"fmt"
	"os"
	"time"
	"flag"
	"strings"

	"net/http"
	"math/rand"
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
		if strings.Contains(err.Error(), "Client.Timeout") {
			return nil
		} else {
			panic(fmt.Sprint("client do error:", err))
		}
	}
	return res
}

var method string
var latency chan float64
var timeout int
var fifo0 = make(chan string)
var die = make(chan bool)

func consume() {
	timeoutDuration := time.Duration(time.Duration(timeout) * time.Second)
	client := &http.Client {
		Timeout: timeoutDuration,
		Transport: &http.Transport {
			DisableKeepAlives: true,
		},
	}
	for url := range fifo0 {
		start := time.Now()
		res := send(client, method, url, "", setReqHeader)
		end := time.Now()
		if res == nil {
			fmt.Printf("HTTP   timeout:     %s %s\n", method, url)
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

	flag.Parse()

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
	}
	avg /= float64(successReq)

	totalReq := total
	fmt.Printf("Trasnaction:                %d hits\n", totalReq)
	fmt.Printf("Availability:               %.2f %%\n", 100 * float64(successReq) / float64(totalReq))
	fmt.Printf("Elapsed time:               %.2f secs\n", eend.Sub(sstart).Seconds())
	//fmt.Println("Data transferred:            ")
	fmt.Printf("Transcation rate:           %.2f trans/sec\n", float64(totalReq) / eend.Sub(sstart).Seconds())
	fmt.Printf("Successful trasnaction:     %d\n", successReq)
	fmt.Printf("Failed trasnaction:         %d\n", totalReq - successReq)
	fmt.Printf("Longest transaction:        %.2f\n", max)
	fmt.Printf("Shortest transaction:       %.2f\n", min)
	fmt.Printf("Average transaction:        %.2f\n", avg)
	fmt.Printf("Throughput:                 %.2f\n", 1.0 / avg * float64(conNum))
}
