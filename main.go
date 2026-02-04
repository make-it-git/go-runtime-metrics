package main

import (
	"log"
	"net/http"
	"runtime"
	"runtime/metrics"
	"sync"
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
	schedLatencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "go_sched_latencies_seconds",
			Help:    "Distribution of goroutine scheduler latencies.",
			Buckets: prometheus.DefBuckets, // Or custom: []float64{0.0001, 0.001, 0.01, 0.1, 1},
		},
		nil,
	)
)

func init() {
	prometheus.MustRegister(mode)
	prometheus.MustRegister(schedLatencies)
}

func pollRuntimeMetrics() {
	ticker := time.NewTicker(time.Second)
	samples := make([]metrics.Sample, 1)
	samples[0].Name = "/sched/latencies:seconds"

	for range ticker.C {
		metrics.Read(samples)

		for _, s := range samples {
			if s.Value.Kind() == metrics.KindFloat64Histogram {
				hist := s.Value.Float64Histogram()

				schedLatencies.Reset()

				totalCount := float64(len(hist.Counts))
				for i, count := range hist.Counts {
					if count > 0 {
						sampleCount := float64(count) / totalCount
						for j := 0; j < int(sampleCount); j++ {
							schedLatencies.With(nil).Observe(hist.Buckets[i])
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
	http.HandleFunc("/cpu-blocking", cpuBlocking)
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

// 2. GC pause pressure (many short-lived allocations)
// curl localhost:8080/gc-pressure
func gcPressure(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("gc_pressure").Set(1)

	go func() {
		for {
			for i := 0; i < 500_000; i++ {
				_ = make([]byte, 1024*1024) // short-lived garbage
			}
			time.Sleep(10 * time.Millisecond)
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

	go func() {
		for {
			buf := make([]byte, 1_000)
			_ = buf
		}
	}()

	w.Write([]byte("started allocation churn\n"))
}

// 5. CPU block
// curl localhost:8080/cpu-blocking
func cpuBlocking(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("cpu_blocking").Set(1)

	runtime.GOMAXPROCS(1)
	var mu sync.Mutex

	go func() {
		for {
			mu.Lock()
			time.Sleep(200 * time.Millisecond)
			mu.Unlock()
		}
	}()

	go func() {
		for {
			go func() {
				mu.Lock()
				mu.Unlock()
			}()
			time.Sleep(5 * time.Millisecond)
		}
	}()

	w.Write([]byte("started CPU blocking\n"))
}

// 6. Syscall pressure
func syscallPressure(w http.ResponseWriter, _ *http.Request) {
	mode.WithLabelValues("syscall_pressure").Set(1)

	go func() {
		for i := 0; i < 1000; i++ { // Spawn 1000 blocking goroutines
			go func() {
				for { // Infinite loop per goroutine
					time.Sleep(100 * time.Millisecond)
				}
			}()
		}
	}()

	w.Write([]byte("started syscall thread pressure\n"))
}
