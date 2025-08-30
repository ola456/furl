package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	hexmatchFlag = flag.Bool("hexmatch", false, "treat -match as hex bytes (e.g. $'\\x1F\\x8B')")
	matchFlag    = flag.String("match", "", "required - regex string (or hex pattern with -hexmatch)")
	optionsFlag  = flag.String("options", "", "curl options, e.g. '-H \"Cookie: 1=1\" -x http://x'")
	p1Flag       = flag.String("p1", "", "required - first/left parts, e.g. 'root urls'")
	p2Flag       = flag.String("p2", "", "required - second/right parts, e.g. paths")
	threadsFlag  = flag.Int("threads", 10, "concurrency/multithreading")
	timeoutFlag  = flag.Int("timeout", 10, "request timeout limit")
)

type Job struct {
	curl string
	regx *regexp.Regexp
	url  string
}

func worker(jobs <-chan Job, wg *sync.WaitGroup) {
	defer wg.Done()

	for j := range jobs {
		// set command time limit
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutFlag)*time.Second)

		// execute the curl command
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", j.curl)
		stdout, err := cmd.Output()
		cancel()

		if err != nil {
			fmt.Printf("error: %v - %v\n", j.url, err)
		}

		// print if match
		var match bool
		if *hexmatchFlag {
			match = bytes.Contains(stdout, []byte(*matchFlag))
		} else {
			match = j.regx.MatchString(string(stdout))
		}
		if match {
			fmt.Printf("match: %v\n", j.url)
		}
	}
}

func main() {
	flag.Parse()

	if *p1Flag == "" || *p2Flag == "" || *matchFlag == "" {
		fmt.Printf("\nfurl v0.0.3 - nothing but a dirty curl wrapper made for cluster bomb fuzzing (with 2 payloads)\n")
		flag.Usage()
		os.Exit(1)
	}

	// fetch first/left parts (p1)
	content, err := os.ReadFile(*p1Flag)
	if err != nil {
		fmt.Printf("error with the p1 wordlist: %v\n", err)
		os.Exit(1)
	}
	p1 := strings.Split(string(content), "\n")
	p1 = p1[:len(p1)-1] // remove trailing newline

	// fetch second/right parts (p2)
	content, err = os.ReadFile(*p2Flag)
	if err != nil {
		fmt.Printf("error with the p2 wordlist: %v\n", err)
		os.Exit(1)
	}
	p2 := strings.Split(string(content), "\n")
	p2 = p2[:len(p2)-1] // remove trailing newline

	// compile regex match
	var regx *regexp.Regexp
	if !*hexmatchFlag {
		// only compile regex if not hexmatch
		regx = regexp.MustCompile(*matchFlag)
	}

	// prepare concurrency
	jobs := make(chan Job, *threadsFlag)
	var wg sync.WaitGroup

	// start workers
	for w := 1; w <= *threadsFlag; w++ {
		wg.Add(1)
		go worker(jobs, &wg)
	}

	// set timeout
	timeoutStr := strconv.Itoa(*timeoutFlag)

	// start testing
	for _, v2 := range p2 {
		for _, v1 := range p1 {
			// craft the curl command
			curl := "curl -sS -kgi --path-as-is '" + v1 + v2 + "' -m " + timeoutStr
			if *optionsFlag != "" {
				curl += " " + *optionsFlag
			}

			jobs <- Job{
				curl: curl,
				regx: regx,
				url:  v1 + v2,
			}
		}
	}
	close(jobs)
	wg.Wait()
}
