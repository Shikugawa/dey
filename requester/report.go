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

package requester

import (
	"io"
	"time"
)

const (
	barChar = "■"
)

// We report for max 1M results.
const maxRes = 1000000

type report struct {
	avgTotal float64
	fastest  float64
	slowest  float64
	average  float64
	rps      float64

	avgConn     float64
	avgDNS      float64
	avgReq      float64
	avgRes      float64
	avgDelay    float64
	connLats    []float64
	dnsLats     []float64
	reqLats     []float64
	resLats     []float64
	delayLats   []float64
	offsets     []float64
	statusCodes []int

	results chan *result
	done    chan bool
	total   time.Duration

	errorDist map[string]int
	lats      []float64
	sizeTotal int64
	numRes    int64
	output    string

	w io.Writer
}

func newReport(w io.Writer, results chan *result, output string, n int) *report {
	cap := min(n, maxRes)
	return &report{
		output:      output,
		results:     results,
		done:        make(chan bool, 1),
		errorDist:   make(map[string]int),
		w:           w,
		connLats:    make([]float64, 0, cap),
		dnsLats:     make([]float64, 0, cap),
		reqLats:     make([]float64, 0, cap),
		resLats:     make([]float64, 0, cap),
		delayLats:   make([]float64, 0, cap),
		lats:        make([]float64, 0, cap),
		statusCodes: make([]int, 0, cap),
	}
}

func runReporter(r *report) {
	// Loop will continue until channel is closed
	for res := range r.results {
		r.numRes++
		if res.err != nil {
			r.errorDist[res.err.Error()]++
		} else {
			r.avgTotal += res.duration.Seconds()
			r.avgConn += res.connDuration.Seconds()
			r.avgDelay += res.delayDuration.Seconds()
			r.avgDNS += res.dnsDuration.Seconds()
			r.avgReq += res.reqDuration.Seconds()
			r.avgRes += res.resDuration.Seconds()
			if len(r.resLats) < maxRes {
				r.lats = append(r.lats, res.duration.Seconds())
				r.connLats = append(r.connLats, res.connDuration.Seconds())
				r.dnsLats = append(r.dnsLats, res.dnsDuration.Seconds())
				r.reqLats = append(r.reqLats, res.reqDuration.Seconds())
				r.delayLats = append(r.delayLats, res.delayDuration.Seconds())
				r.resLats = append(r.resLats, res.resDuration.Seconds())
				r.statusCodes = append(r.statusCodes, res.statusCode)
				r.offsets = append(r.offsets, res.offset.Seconds())
			}
			if res.contentLength > 0 {
				r.sizeTotal += res.contentLength
			}
		}
	}
	// Signal reporter is done.
	r.done <- true
}

func (r *report) finalize(total time.Duration) ServerReport {
	return ServerReport{
		TotalDuration: total,
		AvgTotal:      r.avgTotal,
		Fastest:       r.fastest,
		Slowest:       r.slowest,
		Average:       r.average,
		Rps:           r.rps,
		ContentLength: r.sizeTotal,
		AvgConn:       r.avgConn,
		AvgDNS:        r.avgDNS,
		AvgReq:        r.avgReq,
		AvgRes:        r.avgRes,
		AvgDelay:      r.avgDelay,
		ConnLats:      r.connLats,
		DnsLats:       r.dnsLats,
		ReqLats:       r.reqLats,
		ResLats:       r.resLats,
		DelayLats:     r.delayLats,
		Offsets:       r.offsets,
		StatusCodes:   r.statusCodes,
	}
}

type Report struct {
	AvgTotal float64
	Fastest  float64
	Slowest  float64
	Average  float64
	Rps      float64

	AvgConn  float64
	AvgDNS   float64
	AvgReq   float64
	AvgRes   float64
	AvgDelay float64
	ConnMax  float64
	ConnMin  float64
	DnsMax   float64
	DnsMin   float64
	ReqMax   float64
	ReqMin   float64
	ResMax   float64
	ResMin   float64
	DelayMax float64
	DelayMin float64

	Lats        []float64
	ConnLats    []float64
	DnsLats     []float64
	ReqLats     []float64
	ResLats     []float64
	DelayLats   []float64
	Offsets     []float64
	StatusCodes []int

	Total time.Duration

	ErrorDist      map[string]int
	StatusCodeDist map[int]int
	SizeTotal      int64
	SizeReq        int64
	NumRes         int64

	LatencyDistribution []LatencyDistribution
	Histogram           []Bucket
}

type LatencyDistribution struct {
	Percentage int
	Latency    float64
}

type Bucket struct {
	Mark      float64
	Count     int
	Frequency float64
}
