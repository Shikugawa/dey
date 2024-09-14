// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command hey is an HTTP load generator.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	gourl "net/url"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rakyll/hey/requester"
)

const (
	headerRegexp = `^([\w-]+):\s*(.+)`
	authRegexp   = `^(.+):([^\s].+)`
	heyUA        = "hey/0.0.1"
)

var (
	mode          = flag.String("mode", "", "clienet or server")
	clientTargets = flag.String("client-targets", "", "target urls")
	serverPort    = flag.String("server-port", "", "server port")

	m           = flag.String("m", "GET", "")
	headers     = flag.String("h", "", "")
	body        = flag.String("d", "", "")
	bodyFile    = flag.String("D", "", "")
	accept      = flag.String("A", "", "")
	contentType = flag.String("T", "text/html", "")
	authHeader  = flag.String("a", "", "")
	hostHeader  = flag.String("host", "", "")
	userAgent   = flag.String("U", "", "")

	output = flag.String("o", "", "")

	c = flag.Int("c", 50, "")
	n = flag.Int("n", 200, "")
	q = flag.Float64("q", 0, "")
	t = flag.Int("t", 20, "")
	z = flag.Duration("z", 0, "")

	h2   = flag.Bool("h2", false, "")
	cpus = flag.Int("cpus", runtime.GOMAXPROCS(-1), "")

	disableCompression = flag.Bool("disable-compression", false, "")
	disableKeepAlives  = flag.Bool("disable-keepalive", false, "")
	disableRedirects   = flag.Bool("disable-redirects", false, "")
	proxyAddr          = flag.String("x", "", "")
)

var usage = `Usage: hey [options...] <url>

Options:
  -n  Number of requests to run. Default is 200.
  -c  Number of workers to run concurrently. Total number of requests cannot
      be smaller than the concurrency level. Default is 50.
  -q  Rate limit, in queries per second (QPS) per worker. Default is no rate limit.
  -z  Duration of application to send requests. When duration is reached,
      application stops and exits. If duration is specified, n is ignored.
      Examples: -z 10s -z 3m.
  -o  Output type. If none provided, a summary is printed.
      "csv" is the only supported alternative. Dumps the response
      metrics in comma-separated values format.

  -m  HTTP method, one of GET, POST, PUT, DELETE, HEAD, OPTIONS.
  -H  Custom HTTP header. You can specify as many as needed by repeating the flag.
      For example, -H "Accept: text/html" -H "Content-Type: application/xml" .
  -t  Timeout for each request in seconds. Default is 20, use 0 for infinite.
  -A  HTTP Accept header.
  -d  HTTP request body.
  -D  HTTP request body from file. For example, /home/user/file.txt or ./file.txt.
  -T  Content-type, defaults to "text/html".
  -U  User-Agent, defaults to version "hey/0.0.1".
  -a  Basic authentication, username:password.
  -x  HTTP Proxy address as host:port.
  -h2 Enable HTTP/2.

  -host	HTTP Host header.

  -disable-compression  Disable compression.
  -disable-keepalive    Disable keep-alive, prevents re-use of TCP
                        connections between different HTTP requests.
  -disable-redirects    Disable following of HTTP redirects
  -cpus                 Number of used cpu cores.
                        (default for current machine is %d cores)

  -mode                 Client or Server mode
  -client-targets       Dey Server URLs
  -server-port          Server port
`

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(usage, runtime.NumCPU()))
	}

	var hs headerSlice
	flag.Var(&hs, "H", "")
	flag.Parse()

	if mode == nil || *mode == "" {
		usageAndExit("Please specify the mode.")
	}
	if *mode == "client" {
		targetUrls := strings.Split(*clientTargets, ",")
		if len(targetUrls) == 0 {
			usageAndExit("Please specify the target urls.")
		}
		var wg sync.WaitGroup
		var serverReports []requester.ServerReport

		for _, target := range targetUrls {
			wg.Add(1)
			go func(target string) {
				defer wg.Done()

				client := &http.Client{}
				req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/run", target), nil)
				if err != nil {
					fmt.Printf("Error creating request: %s\n", err)
					return
				}

				resp, err := client.Do(req)
				if err != nil {
					fmt.Printf("Error making request: %s\n", err)
					return
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					fmt.Printf("Error reading response body: %s\n", err)
					return
				}

				var serverReport requester.ServerReport
				if err := json.Unmarshal(body, &serverReport); err != nil {
					fmt.Printf("Error unmarshalling response body: %s\n", err)
					return
				}
				serverReports = append(serverReports, serverReport)
			}(target)
		}

		wg.Wait()
		requester.PrintReport(requester.GenClientReport(serverReports))

		return
	}

	if *mode == "server" {
		runtime.GOMAXPROCS(*cpus)
		num := *n
		conc := *c
		q := *q
		dur := *z

		if dur > 0 {
			num = math.MaxInt32
			if conc <= 0 {
				usageAndExit("-c cannot be smaller than 1.")
			}
		} else {
			if num <= 0 || conc <= 0 {
				usageAndExit("-n and -c cannot be smaller than 1.")
			}

			if num < conc {
				usageAndExit("-n cannot be less than -c.")
			}
		}

		url := flag.Args()[0]
		method := strings.ToUpper(*m)

		// set content-type
		header := make(http.Header)
		header.Set("Content-Type", *contentType)
		// set any other additional headers
		if *headers != "" {
			usageAndExit("Flag '-h' is deprecated, please use '-H' instead.")
		}
		// set any other additional repeatable headers
		for _, h := range hs {
			match, err := parseInputWithRegexp(h, headerRegexp)
			if err != nil {
				usageAndExit(err.Error())
			}
			header.Set(match[1], match[2])
		}

		if *accept != "" {
			header.Set("Accept", *accept)
		}

		// set basic auth if set
		var username, password string
		if *authHeader != "" {
			match, err := parseInputWithRegexp(*authHeader, authRegexp)
			if err != nil {
				usageAndExit(err.Error())
			}
			username, password = match[1], match[2]
		}

		var bodyAll []byte
		if *body != "" {
			bodyAll = []byte(*body)
		}
		if *bodyFile != "" {
			slurp, err := ioutil.ReadFile(*bodyFile)
			if err != nil {
				errAndExit(err.Error())
			}
			bodyAll = slurp
		}

		var proxyURL *gourl.URL
		if *proxyAddr != "" {
			var err error
			proxyURL, err = gourl.Parse(*proxyAddr)
			if err != nil {
				usageAndExit(err.Error())
			}
		}

		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			usageAndExit(err.Error())
		}
		req.ContentLength = int64(len(bodyAll))
		if username != "" || password != "" {
			req.SetBasicAuth(username, password)
		}

		// set host header if set
		if *hostHeader != "" {
			req.Host = *hostHeader
		}

		ua := header.Get("User-Agent")
		if ua == "" {
			ua = heyUA
		} else {
			ua += " " + heyUA
		}
		header.Set("User-Agent", ua)

		// set userAgent header if set
		if *userAgent != "" {
			ua = *userAgent + " " + heyUA
			header.Set("User-Agent", ua)
		}

		req.Header = header

		// TODO: 同時実行数を1にする
		handler := func(rw http.ResponseWriter, r *http.Request) {
			w := &requester.Work{
				Request:            req,
				RequestBody:        bodyAll,
				N:                  num,
				C:                  conc,
				QPS:                q,
				Timeout:            *t,
				DisableCompression: *disableCompression,
				DisableKeepAlives:  *disableKeepAlives,
				DisableRedirects:   *disableRedirects,
				H2:                 *h2,
				ProxyAddr:          proxyURL,
				Output:             "csv",
			}
			w.Init()

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			go func() {
				<-c
				w.Stop()
			}()
			if dur > 0 {
				go func() {
					time.Sleep(dur)
					w.Stop()
				}()
			}
			servReport := w.Run()

			if raw, err := json.Marshal(servReport); err != nil {
				fmt.Fprintf(os.Stderr, "Error marshalling report: %v\n", err)
			} else {
				rw.WriteHeader(http.StatusOK)
				rw.Write(raw)
			}
		}

		var port string
		if *serverPort != "" {
			port = fmt.Sprintf(":%s", *serverPort)
		} else {
			port = ":8081"
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/run", handler)

		server := &http.Server{
			Addr:    port,
			Handler: mux,
		}

		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			fmt.Println("Starting server...")
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Printf("Error starting server: %s\n", err)
			}
		}()

		<-stop

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fmt.Println("Shutting down server...")
		if err := server.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down server: %s\n", err)
		}
		fmt.Println("Server gracefully stopped")
	}
}

func errAndExit(msg string) {
	fmt.Fprintf(os.Stderr, msg)
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

func usageAndExit(msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, msg)
		fmt.Fprintf(os.Stderr, "\n\n")
	}
	flag.Usage()
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

func parseInputWithRegexp(input, regx string) ([]string, error) {
	re := regexp.MustCompile(regx)
	matches := re.FindStringSubmatch(input)
	if len(matches) < 1 {
		return nil, fmt.Errorf("could not parse the provided input; input = %v", input)
	}
	return matches, nil
}

type headerSlice []string

func (h *headerSlice) String() string {
	return fmt.Sprintf("%s", *h)
}

func (h *headerSlice) Set(value string) error {
	*h = append(*h, value)
	return nil
}
