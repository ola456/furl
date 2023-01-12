package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"strconv"
)

var (
	matchFlag   = flag.String("match", "", "required - regex string to match on")
	optionsFlag = flag.String("options", "", "curl options, e.g. '-H \"Cookie: 1=1\" -x http://x'")
	p1Flag      = flag.String("p1", "", "required - first/left parts, e.g. 'root urls'")
	p2Flag      = flag.String("p2", "", "required - second/right parts, e.g. paths")
	threadsFlag = flag.Int("threads", 10, "concurrency/multithreading")
	timeoutFlag = flag.Int("timeout", 10, "request timeout limit")
)

type Job struct {
	curl string
	regx *regexp.Regexp
	url string
}

func worker(jobs <-chan Job) {
    for j := range jobs {
		// set command time limit
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutFlag) * time.Second)
		defer cancel()

		// execute the curl command
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", j.curl)
		stdout, err := cmd.Output()
		if err != nil {
			fmt.Printf("error: %v - %v\n", j.url, err)
		}
	
		// print if match
		match := j.regx.MatchString(string(stdout))
		if match {
			fmt.Printf("match: %v\n", j.url)
		}

        time.Sleep(time.Millisecond) // this is prob. unessesaaryyy
    }
}

func main() {
	flag.Parse()

	if *p1Flag == "" || *p1Flag == "" || *matchFlag == "" {
		fmt.Println("\nfurl v0.0.1 - nothing but a dirty curl wrapper made for cluster bomb fuzzing (with 2 payloads)\n")
		flag.Usage()
		os.Exit(1)
	}

	// fetch first/left parts (p1)
	content, err := ioutil.ReadFile(*p1Flag)
	if err != nil {
		fmt.Printf("error with the p1 wordlist: %v", err)
		os.Exit(1)
	}
	p1 := strings.Split(string(content), "\n")
	p1 = p1[:len(p1) - 1] // remove trailing newline

	// fetch second/right parts (p2)
	content, err = ioutil.ReadFile(*p2Flag)
	if err != nil {
		fmt.Printf("error with the p2 wordlist: %v", err)
		os.Exit(1)
	}
	p2 := strings.Split(string(content), "\n")
	p2 = p2[:len(p2) - 1] // remove trailing newline

	// compile the regex match
	regx := regexp.MustCompile(*matchFlag)

	// prepare concurrency
	jobs := make(chan Job, *threadsFlag)
	for w := 1; w <= *threadsFlag; w++ {
		go worker(jobs)
	}

	// start testing
	for _, v2 := range p2 {
		for _, v1 := range p1 {
			// craft the curl command
			curl := "curl -sS -kgi --path-as-is '" + v1 + v2 + "' -m " + strconv.Itoa(*timeoutFlag)
			if *optionsFlag != "" {
				curl += " " + *optionsFlag
			}

			jobs <- Job{
				curl: curl,
				regx: regx,
				url: v1 + v2,
			}
		}
	}
	close(jobs)
}
