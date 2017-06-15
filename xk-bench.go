package main

import (
	_"net"
	"bufio"
	"fmt"
	"os"
	"time"
	"flag"
	"runtime"
	"strings"

	"net/http"
	"math/rand"

	_"github.com/crazyboycjr/mobike-ofo-crawler/utility"
)

//const conNum int = 1

func setReqHeader(req *http.Request) {
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept-Language", "en-us")
	req.Header.Set("Aceept", "*/*")
}

func newRequest(method, url, data string) *http.Request {
	body := strings.NewReader(data)
	for {
		req, err := http.NewRequest(method, url, body)
		if err != nil {
			fmt.Println("http new request error:", err)
			//time.Sleep(time.Millisecond * time.Duration(rand.Intn(500) + 100))
		} else {
			return req
		}
	}
}

func send(client *http.Client, method, url, data string, setHeader func(req *http.Request)) *http.Response {
	for {
		req := newRequest(method, url, data)
		setHeader(req)
		res, err := client.Do(req)
		if err != nil {
			fmt.Println("client do error:", err)
		} else {
			if res.StatusCode != 200 {
				fmt.Println("response status = ", res.Status)
			}
			return res
		}
		return res
		//time.Sleep(time.Millisecond * time.Duration(rand.Intn(500) + 1000))
	}
}

var method string
var latency = make([]float64, 0)
var successReq int = 0
var totalReq int = 0
var timeout int

func consume(fifo0 chan string, c chan int) {
	for url := range fifo0 {
		timeoutDuration := time.Duration(time.Duration(timeout) * time.Second)
		client := &http.Client {
			Timeout: timeoutDuration,
		}
		//text := utility.ReadAll(client, url, "", setReqHeader)
		//fmt.Println(string(text))
		start := time.Now()
		res := send(client, method, url, "", setReqHeader)
		end := time.Now()
		totalReq++
		c <- 1
		if res == nil {
			fmt.Printf("HTTP   timeout:     %s %s\n", method, url)
			continue
		}
		delta := end.Sub(start).Seconds()
		if res.StatusCode < 500 {
			latency = append(latency, delta)
		}
		if res.StatusCode == 200 {
			successReq++
		}
		fmt.Printf("HTTP %d   %.2f secs:     %s %s\n", res.StatusCode, delta, method, url)
		res.Body.Close()
	}
}

func main() {
	runtime.GOMAXPROCS(1)
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

	sstart := time.Now()
	fifo0 := make(chan string)

	c := make(chan int)
	for i := 0; i < conNum; i++ {
		go consume(fifo0, c)
	}

	var cnt int
	var cnt2 int
	for cnt, cnt2 = 0, 0; cnt2 < total; {
		num := rand.Intn(len(lines))
		if cnt < total {
			select {
			case fifo0 <- lines[num]:
				cnt++
			case <-c:
				cnt2++
			}
		} else {
			<-c
			cnt2++
		}
	}
	eend := time.Now()

	max := 0.0
	min := 1000000.0
	avg := 0.0
	for _, d := range latency {
		if max < d {
			max = d
		}
		if min > d {
			min = d
		}
		avg += d
	}
	avg /= float64(len(latency))

	time.Sleep(1 * time.Second)
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
