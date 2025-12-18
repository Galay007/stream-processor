package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	RedisKeyPrefix  = "stream:data:"
	AvgWindowSize   = 50
	ZScoreThreshold = 2.0
)

var (
	ctx         = context.Background()
	RedisClient *redis.Client
	Detector    *AnomalyDetector
)

type DataPoint struct {
	DeviceID  string  `json:"device_id"`
	Timestamp int64   `json:"timestamp"`
	CPU       float64 `json:"cpu"`
	RPS       float64 `json:"rps"`
}

func Initialize(redisAddr string) {

	RedisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}
	log.Println("Successfully connected to Redis")

	Detector = NewAnomalyDetector(AvgWindowSize, ZScoreThreshold)

	log.Printf("Anomaly Detector initialized with window size %d and threshold %.1f", AvgWindowSize, ZScoreThreshold)
}

func HandleAnalyze(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceID := vars["device_id"]

	if deviceID == "" {
		http.Error(w, "Device ID required", http.StatusBadRequest)
		return
	}

	key := RedisKeyPrefix + deviceID
	lastValueStr, err := RedisClient.Get(ctx, key).Result()

	stats := Detector.GetOrCreateStats(deviceID, AvgWindowSize)

	stats.mu.Lock()
	avg := stats.LastAvg
	stdDev := stats.stdDev
	zScore := stats.LastZScore
	anomalyCount := stats.AnomalyCountTotal
	stats.mu.Unlock()

	response := struct {
		DeviceID       string   `json:"device_id"`
		LastValue      *float64 `json:"last_value,omitempty"`
		RollingAverage float64  `json:"rolling_average"`
		StdDev         float64  `json:"std_dev"`
		ZScore         float64  `json:"z_score"`
		AnomalyCount   int64    `json:"anomaly_count"`
		Status         string   `json:"status"`
	}{
		DeviceID:       deviceID,
		RollingAverage: avg,
		StdDev:         stdDev,
		ZScore:         zScore,
		AnomalyCount:   anomalyCount,
	}

	if err == redis.Nil {
		response.Status = "No data found in Redis"
	} else if err != nil {
		log.Printf("Error retrieving data from Redis for %s: %v", deviceID, err)
		http.Error(w, "Internal error retrieving data", http.StatusInternalServerError)
		return
	} else {
		lastValue, _ := strconv.ParseFloat(lastValueStr, 64)
		response.LastValue = &lastValue
		response.Status = "OK"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func HandleStreamData(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(Metrics.RequestDuration.WithLabelValues("/stream"))

	var dp DataPoint
	if err := json.NewDecoder(r.Body).Decode(&dp); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		Metrics.TotalRequestsCounter.WithLabelValues("/stream", "error").Inc()
		timer.ObserveDuration()
		return
	}

	if dp.DeviceID == "" {
		http.Error(w, "device_id is required", http.StatusBadRequest)
		Metrics.TotalRequestsCounter.WithLabelValues("/stream", "no_device").Inc()
		timer.ObserveDuration()
		return
	}

	go func(dp DataPoint) {
		key := RedisKeyPrefix + dp.DeviceID
		err := RedisClient.Set(ctx, key, strconv.FormatFloat(dp.CPU, 'f', -1, 64), time.Minute*15).Err()
		if err != nil {
			log.Printf("Error saving to Redis: %v", err)
		}
	}(dp)

	go Detector.ProcessData(dp.DeviceID, dp.CPU)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Data accepted and processing initiated"))
	timer.ObserveDuration()

	Metrics.TotalRequestsCounter.WithLabelValues("/stream", "success").Inc()
	Metrics.CurrentRPS.WithLabelValues(dp.DeviceID).Set(dp.RPS)
}

func HandleMetrics(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
}

func HandleStatus(w http.ResponseWriter, r *http.Request) {
	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		http.Error(w, "Redis connection failed", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("Service is up and Redis is connected"))
}

func SetupRoutes() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/stream", HandleStreamData).Methods("POST")
	r.HandleFunc("/analyze/{device_id}", HandleAnalyze).Methods("GET")
	r.HandleFunc("/metrics", HandleMetrics).Methods("GET")
	r.HandleFunc("/status", HandleStatus).Methods("GET")
	return r
}
