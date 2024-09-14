package requester

import (
	"sort"
	"time"
)

type ServerReport struct {
	// Set longest duration from each server work
	TotalDuration time.Duration `json:"totalDuration"`
	AvgTotal      float64       `json:"avgTotal"`
	Fastest       float64       `json:"fastest"`
	Slowest       float64       `json:"slowest"`
	Average       float64       `json:"average"`
	Rps           float64       `json:"rps"`
	ContentLength int64         `json:"contentLength"`
	AvgConn       float64       `json:"avgConn"`
	AvgDNS        float64       `json:"avgDNS"`
	AvgReq        float64       `json:"avgReq"`
	AvgRes        float64       `json:"avgRes"`
	AvgDelay      float64       `json:"avgDelay"`
	ConnLats      []float64     `json:"connLats"`
	DnsLats       []float64     `json:"dnsLats"`
	ReqLats       []float64     `json:"reqLats"`
	ResLats       []float64     `json:"resLats"`
	DelayLats     []float64     `json:"delayLats"`
	Offsets       []float64     `json:"offsets"`
	StatusCodes   []int         `json:"statusCodes"`
}

func GenClientReport(reps []ServerReport) Report {
	snapshot := Report{}
	snapshot.AvgTotal = func() float64 {
		var sum float64
		for _, rep := range reps {
			sum += rep.AvgTotal
		}
		return sum / float64(len(reps))
	}()
	snapshot.Average = func() float64 {
		var sum float64
		for _, rep := range reps {
			sum += rep.Average
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
	snapshot.Lats = []float64{}
	for _, rep := range reps {
		snapshot.Lats = append(snapshot.Lats, rep.Fastest)
	}
	snapshot.ConnLats = []float64{}
	for _, rep := range reps {
		snapshot.ConnLats = append(snapshot.ConnLats, rep.AvgConn)
	}
	snapshot.DnsLats = []float64{}
	for _, rep := range reps {
		snapshot.DnsLats = append(snapshot.DnsLats, rep.AvgDNS)
	}
	snapshot.ReqLats = []float64{}
	for _, rep := range reps {
		snapshot.ReqLats = append(snapshot.ReqLats, rep.AvgReq)
	}
	snapshot.ResLats = []float64{}
	for _, rep := range reps {
		snapshot.ResLats = append(snapshot.ResLats, rep.AvgRes)
	}
	snapshot.DelayLats = []float64{}
	for _, rep := range reps {
		snapshot.DelayLats = append(snapshot.DelayLats, rep.AvgDelay)
	}
	snapshot.StatusCodes = []int{}
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

	lats := []float64{}
	copy(lats, snapshot.Lats)
	sort.Float64s(lats)
	snapshot.Fastest = lats[0]
	snapshot.Slowest = lats[len(lats)-1]

	connLats := []float64{}
	copy(connLats, snapshot.ConnLats)
	sort.Float64s(connLats)
	snapshot.ConnMax = connLats[0]
	snapshot.ConnMin = connLats[len(connLats)-1]

	dnsLats := []float64{}
	copy(dnsLats, snapshot.DnsLats)
	sort.Float64s(dnsLats)
	snapshot.DnsMax = dnsLats[0]
	snapshot.DnsMin = dnsLats[len(dnsLats)-1]

	reqLats := []float64{}
	copy(reqLats, snapshot.ReqLats)
	sort.Float64s(reqLats)
	snapshot.ReqMax = reqLats[0]
	snapshot.ReqMin = reqLats[len(reqLats)-1]

	delayLats := []float64{}
	copy(delayLats, snapshot.DelayLats)
	sort.Float64s(delayLats)
	snapshot.DelayMax = delayLats[0]
	snapshot.DelayMin = delayLats[len(delayLats)-1]

	resLats := []float64{}
	copy(resLats, snapshot.ResLats)
	sort.Float64s(resLats)
	snapshot.ResMax = resLats[0]
	snapshot.ResMin = resLats[len(resLats)-1]

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
