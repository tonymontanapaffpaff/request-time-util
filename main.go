package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

type RequestTime struct {
	URL      string
	Duration time.Duration
	Err      error
}

var totalTimeDuration time.Duration

func DoRequest(urlStr string, ch chan RequestTime) {
	startTime := time.Now()

	var requestTime = RequestTime{URL: urlStr}

	// create a new client, disable redirections
	var client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// create new request
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		fmt.Printf("Error: cannot create http request: %v\n", err)
		requestTime.Duration = time.Since(startTime)
		requestTime.Err = err
		ch <- requestTime
		return
	}

	// launch request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("cannot launch request: %v\n", err)
		requestTime.Duration = time.Since(startTime)
		requestTime.Err = err
		ch <- requestTime
		return
	}

	defer client.CloseIdleConnections()

	// read body
	if _, err := ioutil.ReadAll(resp.Body); err != nil {
		fmt.Println(err)
		requestTime.Duration = time.Since(startTime)
		requestTime.Err = err
		ch <- requestTime
		return
	}

	defer resp.Body.Close()

	// execute duration and send requestTime
	timeDuration := time.Since(startTime)
	requestTime.Duration = timeDuration
	totalTimeDuration += timeDuration
	ch <- requestTime
}

func GetRequestTimes(urls []string, timeout time.Duration) []RequestTime {

	var requestTimes []RequestTime
	requestTimeCh := make(chan RequestTime)

	// start parallel execution
	for _, thisURL := range urls {
		go DoRequest(thisURL, requestTimeCh)
	}

	// execute timeout
	timeoutChannel := time.After(timeout * time.Millisecond)

	// get requestTimes depending on the elapsed time
	for i := 0; i < len(urls); i++ {
		select {
		case <-timeoutChannel:
			fmt.Printf("%s: timeout\n", urls[i])
		case requestTime := <-requestTimeCh:
			requestTimes = append(requestTimes, requestTime)
		}
	}

	return requestTimes
}

func minTimeDuration(values []time.Duration) time.Duration {
	var min time.Duration
	for i, val := range values {
		if i == 0 || val < min {
			min = val
		}
	}
	return min.Truncate(time.Millisecond)
}

func maxTimeDuration(values []time.Duration) time.Duration {
	var max time.Duration
	for i, val := range values {
		if i == 0 || val > max {
			max = val
		}
	}
	return max.Truncate(time.Millisecond)
}

func avgTimeDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return time.Duration(0)
	}

	var total time.Duration
	for _, val := range values {
		total += val
	}
	return time.Duration(float64(total) / float64(len(values))).Truncate(time.Millisecond)
}

func main() {

	// command line flags
	var count, timeout int

	flag.IntVar(&count, "c", 3, "number of requests")
	flag.IntVar(&timeout, "t", 1000, "timeout in ms")

	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("Error: not enough arguments")
		os.Exit(1)
	}

	var urls []string

	// check urls in command line args
	for _, arg := range flag.Args() {
		thisURL, err := url.Parse(arg)

		if err != nil {
			fmt.Printf("Error: cannot parse %v as url: %v\n", arg, err)
			os.Exit(1)
		}

		if thisURL.Scheme != "http" && thisURL.Scheme != "https" {
			fmt.Printf("Error: unsupported url scheme in %v\n", arg)
			os.Exit(1)
		}

		urls = append(urls, thisURL.String())
	}

	var countRequests = count * len(urls)
	var countTimeout int

	// a slice to contain results
	var timeDurations []time.Duration

	for i := 0; i < count; i++ {
		results := GetRequestTimes(urls, time.Duration(timeout))
		var duration time.Duration

		for _, result := range results {
			fmt.Printf("%v: %v\n", result.URL, result.Duration)
			duration += result.Duration
		}

		countTimeout += len(urls) - len(results)

		if duration > 0 {
			timeDurations = append(timeDurations, duration)
		}
	}

	fmt.Println("Summary")
	fmt.Printf("received: %d/%d\ntimeout: %d (%.02f%%)\ntotal time: %v\nmin: %v\nmax: %v\navg: %v\n",
		countRequests - countTimeout, countRequests, countTimeout, float64(countTimeout*100)/float64(countRequests),
		totalTimeDuration.Truncate(time.Millisecond), minTimeDuration(timeDurations), maxTimeDuration(timeDurations),
		avgTimeDuration(timeDurations))
}