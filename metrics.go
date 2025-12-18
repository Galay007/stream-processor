package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type CustomMetrics struct {
	TotalRequestsCounter *prometheus.CounterVec
	RequestDuration      *prometheus.HistogramVec
	AnomalyTotalCounter  *prometheus.CounterVec
	CurrentRPS           *prometheus.GaugeVec
}

var Metrics CustomMetrics

func init() {
	Metrics.TotalRequestsCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stream_processor_request_total",
			Help: "Total number of stream data points processed.",
		},
		[]string{"endpoint", "status"},
	)

	Metrics.RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "stream_processor_request_duration_seconds",
			Help:    "Processing time of stream requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint"},
	)

	Metrics.CurrentRPS = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "stream_processor_current_rps",
			Help: "Requests processed per second.",
		},
		[]string{"device_id"},
	)

	Metrics.AnomalyTotalCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stream_processor_anomaly_counter",
			Help: "Total cumulative count of detected anomalies.",
		},
		[]string{"device_id"},
	)

}
