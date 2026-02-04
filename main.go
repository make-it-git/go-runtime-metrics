package main

import (
	"log"
	"net/http"
	"runtime/metrics"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	mode = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "demo_mode",
			Help: "Which pathological mode is enabled",
		},
		[]string{"name"},
	)
	schedLatencies = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "go",
			Subsystem: "scheduler",
			Name:      "latency_seconds",
			Help:      "Time goroutines spend waiting to be scheduled.",
			Buckets:   prometheus.ExponentialBuckets(1e-6, 2, 20),
		},
	)
)

func init() {
	prometheus.MustRegister(mode)
	prometheus.MustRegister(schedLatencies)
}

func pollRuntimeMetrics() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	samples := []metrics.Sample{
		{Name: "/sched/latencies:seconds"},
	}

	for range ticker.C {
		metrics.Read(samples)

		for _, s := range samples {
			if s.Value.Kind() == metrics.KindFloat64Histogram {
				hist := s.Value.Float64Histogram()
				totalCount := float64(len(hist.Counts))
				for i, count := range hist.Counts {
					if count > 0 {
						sampleCount := float64(count) / totalCount
						for j := 0; j < int(sampleCount); j++ {
							schedLatencies.Observe(hist.Buckets[i])
						}
					}
				}
				break
			}
		}
	}
}

func main() {
	go pollRuntimeMetrics()

	// HTTP endpoints to trigger patterns
	http.HandleFunc("/leak-goroutines", leakGoroutines)
	http.HandleFunc("/gc-pressure", gcPressure)
	http.HandleFunc("/memory-growth", memoryGrowth)
	http.HandleFunc("/alloc-churn", allocChurn)
	http.HandleFunc("/syscall-pressure", syscallPressure)

	http.Handle("/metrics", promhttp.Handler())

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// 1. Goroutine leak
// curl localhost:8080/leak-goroutines
func leakGoroutines(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("goroutine_leak").Set(1)

	go func() {
		for {
			go func() {
				select {} // never exits
			}()
			time.Sleep(100 * time.Millisecond)
		}
	}()

	w.Write([]byte("started goroutine leak\n"))
}

type Node struct {
	next *Node
	data []byte
}

func allocChain(n int) *Node {
	var head *Node
	for i := 0; i < n; i++ {
		head = &Node{
			next: head,
			data: make([]byte, 1024),
		}
	}
	return head
}

// 2. GC pause pressure (many short-lived allocations)
// curl localhost:8080/gc-pressure
func gcPressure(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("gc_pressure").Set(1)

	var roots []*Node

	go func() {
		for {
			roots = append(roots, allocChain(100_000))
			if len(roots) > 10 {
				roots = roots[:0]
			}
		}
	}()

	w.Write([]byte("started GC pressure\n"))
}

// 3. Heap + RSS growth (long-lived objects)
// curl localhost:8080/memory-growth
func memoryGrowth(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("memory_growth").Set(1)

	var store [][]byte
	go func() {
		for {
			store = append(store, make([]byte, 1_000_000)) // retained
			time.Sleep(500 * time.Millisecond)
		}
	}()

	w.Write([]byte("started memory growth\n"))
}

// 4. Allocation churn without heap growth
// curl localhost:8080/alloc-churn
func allocChurn(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("alloc_churn").Set(1)

	var globalSink *[]byte
	_ = globalSink

	go func() {
		for {
			buf := make([]byte, 1_000) // moved to heap: buf; go build -gcflags="-m"
			globalSink = &buf
		}
	}()

	w.Write([]byte("started allocation churn\n"))
}

// 5. Syscall pressure
// curl localhost:8080/syscall-pressure
func syscallPressure(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("syscall_pressure").Set(1)

	go func() {
		for i := 0; i < 100_000; i++ {
			go func() {
				for {
					time.Sleep(20 * time.Millisecond)
				}
			}()
		}
	}()

	w.Write([]byte("started syscall thread pressure\n"))
}
