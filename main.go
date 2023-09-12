package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics
var (
	jobQueue              = promauto.NewGauge(prometheus.GaugeOpts{Name: "workerpool_jobs_queued", Help: "Number of jobs currently in the queue"})
	activeWorkers         = promauto.NewGauge(prometheus.GaugeOpts{Name: "workerpool_active_workers", Help: "Number of currently active workers"})
	jobState              = promauto.NewGaugeVec(prometheus.GaugeOpts{Name: "job_state", Help: "The state of the job"}, []string{"jobID", "state"})
	jobProcessingDuration = promauto.NewHistogram(prometheus.HistogramOpts{Name: "job_processing_duration_seconds", Help: "The duration it takes to process a job", Buckets: prometheus.LinearBuckets(1, 1, 10)})
)

// Job represents a unit of work
type Job struct {
	ID    string
	Data  string
	State string // states can be "Queued", "Processing", "Completed"
}

// processJob simulates processing a job by a worker
func worker(id int, jobs <-chan *Job, done chan<- *Job) {
	for job := range jobs {
		startTime := time.Now()

		activeWorkers.Inc()
		updateJobState(job, "Processing")

		// Simulating job processing
		time.Sleep(time.Duration(rand.Intn(10000)+100) * time.Millisecond)

		activeWorkers.Dec()
		jobProcessingDuration.Observe(time.Since(startTime).Seconds())
		updateJobState(job, "Completed")

		done <- job
	}
}

// updateJobState updates the state of the job and sets Prometheus metrics accordingly
func updateJobState(job *Job, state string) {
	job.State = state
	jobState.WithLabelValues(job.ID, state).Set(1)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	// Setup HTTP server for Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":8080", nil)

	const numWorkers = 99
	const numJobs = 10000
	jobs := make(chan *Job, numJobs)
	done := make(chan *Job)
	var wg sync.WaitGroup

	for i := 1; i <= numWorkers; i++ {
		go worker(i, jobs, done)
	}

	wg.Add(numJobs)
	// Assign jobs using goroutines
	for i := 1; i <= numJobs; i++ {
		go assignJob(i, jobs, done, &wg)
	}

	wg.Wait()
	time.Sleep(1 * time.Minute) // Allow time for Prometheus to scrape metrics
}

// assignJob creates a new job, sends it for processing, and waits for it to complete
func assignJob(i int, jobs chan<- *Job, done <-chan *Job, wg *sync.WaitGroup) {
	defer wg.Done()

	jobID := fmt.Sprintf("%s-%d", uuid.New().String(), i)
	job := &Job{ID: jobID, Data: fmt.Sprintf("Data_%d", i), State: "Queued"}
	updateJobState(job, "Queued")

	jobQueue.Inc()
	jobs <- job
	<-done
	jobQueue.Dec()
}
