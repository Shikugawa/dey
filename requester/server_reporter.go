package requester

import (
	"fmt"
	"sort"
	"time"
)

type ServerReport struct {
	// Set longest duration from each server work
	TotalDuration time.Duration `json:"totalDuration"`
	AvgTotal      float64       `json:"avgTotal"`
	Rps           float64       `json:"rps"`
	ContentLength int64         `json:"contentLength"`
	AvgConn       float64       `json:"avgConn"`
	AvgDNS        float64       `json:"avgDNS"`
	AvgReq        float64       `json:"avgReq"`
	AvgRes        float64       `json:"avgRes"`
	AvgDelay      float64       `json:"avgDelay"`
	Lats          []float64     `json:"lats"`
	ConnLats      []float64     `json:"connLats"`
	DnsLats       []float64     `json:"dnsLats"`
	ReqLats       []float64     `json:"reqLats"`
	ResLats       []float64     `json:"resLats"`
	DelayLats     []float64     `json:"delayLats"`
	Offsets       []float64     `json:"offsets"`
	StatusCodes   []int         `json:"statusCodes"`

	Errors map[string]int `json:"errors"`
}

func GenClientReport(reps []ServerReport) Report {
	// TODO: 適当
	var isError bool
	for _, rep := range reps {
		if len(rep.Errors) > 0 {
			for k := range rep.Errors {
				fmt.Println("Error:", k)
			}
			isError = true
			break
		}
	}
	if isError {
		return Report{}
	}

	snapshot := Report{}
	snapshot.AvgTotal = func() float64 {
		var sum float64
		for _, rep := range reps {
			sum += rep.AvgTotal
		}
		return sum / float64(len(reps))
	}()
	// In distributed manner, it's average of servers
	snapshot.Rps = func() float64 {
		var sum float64
		for _, rep := range reps {
			sum += rep.Rps
		}
		return sum / float64(len(reps))
	}()
	snapshot.SizeTotal = func() int64 {
		var sum int64
		for _, rep := range reps {
			sum += rep.ContentLength
		}
		return sum
	}()
	snapshot.AvgConn = func() float64 {
		var sum float64
		for _, rep := range reps {
			sum += rep.AvgConn
		}
		return sum / float64(len(reps))
	}()
	snapshot.AvgDNS = func() float64 {
		var sum float64
		for _, rep := range reps {
			sum += rep.AvgDNS
		}
		return sum / float64(len(reps))
	}()
	snapshot.AvgReq = func() float64 {
		var sum float64
		for _, rep := range reps {
			sum += rep.AvgReq
		}
		return sum / float64(len(reps))
	}()
	snapshot.AvgRes = func() float64 {
		var sum float64
		for _, rep := range reps {
			sum += rep.AvgRes
		}
		return sum / float64(len(reps))
	}()
	snapshot.AvgDelay = func() float64 {
		var sum float64
		for _, rep := range reps {
			sum += rep.AvgDelay
		}
		return sum / float64(len(reps))
	}()
	snapshot.Total = 0 // Not used
	for _, rep := range reps {
		snapshot.Lats = append(snapshot.Lats, rep.Lats...)
	}
	// snapshot.Lats = rep
	for _, rep := range reps {
		snapshot.ConnLats = append(snapshot.ConnLats, rep.ConnLats...)
	}
	for _, rep := range reps {
		snapshot.DnsLats = append(snapshot.DnsLats, rep.DnsLats...)
	}
	for _, rep := range reps {
		snapshot.ReqLats = append(snapshot.ReqLats, rep.ReqLats...)
	}
	for _, rep := range reps {
		snapshot.ResLats = append(snapshot.ResLats, rep.ResLats...)
	}
	for _, rep := range reps {
		snapshot.DelayLats = append(snapshot.DelayLats, rep.DelayLats...)
	}
	for _, rep := range reps {
		snapshot.StatusCodes = append(snapshot.StatusCodes, rep.StatusCodes...)
	}
	snapshot.Offsets = []float64{}
	for _, rep := range reps {
		snapshot.Offsets = append(snapshot.Offsets, rep.Offsets...)
	}
	for _, rep := range reps {
		if rep.TotalDuration > snapshot.Total {
			snapshot.Total = rep.TotalDuration
		}
	}

	sort.Float64s(snapshot.Lats)
	snapshot.Fastest = snapshot.Lats[0]
	snapshot.Slowest = snapshot.Lats[len(snapshot.Lats)-1]

	sort.Float64s(snapshot.ConnLats)
	snapshot.ConnMax = snapshot.ConnLats[0]
	snapshot.ConnMin = snapshot.ConnLats[len(snapshot.ConnLats)-1]

	sort.Float64s(snapshot.DnsLats)
	snapshot.DnsMax = snapshot.DnsLats[0]
	snapshot.DnsMin = snapshot.DnsLats[len(snapshot.DnsLats)-1]

	sort.Float64s(snapshot.ReqLats)
	snapshot.ReqMax = snapshot.ReqLats[0]
	snapshot.ReqMin = snapshot.ReqLats[len(snapshot.ReqLats)-1]

	sort.Float64s(snapshot.DelayLats)
	snapshot.DelayMax = snapshot.DelayLats[0]
	snapshot.DelayMin = snapshot.DelayLats[len(snapshot.DelayLats)-1]

	sort.Float64s(snapshot.ResLats)
	snapshot.ResMax = snapshot.ResLats[0]
	snapshot.ResMin = snapshot.ResLats[len(snapshot.ResLats)-1]

	snapshot.Histogram = histrgramForClientReport(snapshot)
	snapshot.LatencyDistribution = latenciesForClientReport(snapshot)

	statusCodeDist := make(map[int]int, len(snapshot.StatusCodes))
	for _, statusCode := range snapshot.StatusCodes {
		statusCodeDist[statusCode]++
	}
	snapshot.StatusCodeDist = statusCodeDist

	return snapshot
}

func latenciesForClientReport(snapshot Report) []LatencyDistribution {
	pctls := []int{10, 25, 50, 75, 90, 95, 99}
	data := make([]float64, len(pctls))
	j := 0
	for i := 0; i < len(snapshot.Lats) && j < len(pctls); i++ {
		current := i * 100 / len(snapshot.Lats)
		if current >= pctls[j] {
			data[j] = snapshot.Lats[i]
			j++
		}
	}
	res := make([]LatencyDistribution, len(pctls))
	for i := 0; i < len(pctls); i++ {
		if data[i] > 0 {
			res[i] = LatencyDistribution{Percentage: pctls[i], Latency: data[i]}
		}
	}
	return res
}

func histrgramForClientReport(snapshot Report) []Bucket {
	bc := 10
	buckets := make([]float64, bc+1)
	counts := make([]int, bc+1)
	bs := (snapshot.Slowest - snapshot.Fastest) / float64(bc)
	for i := 0; i < bc; i++ {
		buckets[i] = snapshot.Fastest + bs*float64(i)
	}
	buckets[bc] = snapshot.Slowest
	var bi int
	var max int
	for i := 0; i < len(snapshot.Lats); {
		if snapshot.Lats[i] <= buckets[bi] {
			i++
			counts[bi]++
			if max < counts[bi] {
				max = counts[bi]
			}
		} else if bi < len(buckets)-1 {
			bi++
		}
	}
	res := make([]Bucket, len(buckets))
	for i := 0; i < len(buckets); i++ {
		res[i] = Bucket{
			Mark:      buckets[i],
			Count:     counts[i],
			Frequency: float64(counts[i]) / float64(len(snapshot.Lats)),
		}
	}
	return res
}
