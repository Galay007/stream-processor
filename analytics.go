package main

import (
	"math"
	"sync"
)

type DeviceStats struct {
	data              []float64
	capacity          int
	sum               float64
	stdDev            float64
	LastAvg           float64
	LastZScore        float64
	AnomalyCountTotal int64
	mu                sync.Mutex
}

func NewDeviceStats(capacity int) *DeviceStats {
	return &DeviceStats{
		capacity: capacity,
		data:     make([]float64, 0, capacity),
	}
}

func (ds *DeviceStats) AddAndCalculate(value float64) (avg float64, stdDev float64) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.data = append(ds.data, value)
	ds.sum += value

	if len(ds.data) > ds.capacity {
		oldest := ds.data[0]
		ds.data = ds.data[1:]
		ds.sum -= oldest
	}

	N := float64(len(ds.data))
	avg = ds.sum / N
	ds.LastAvg = avg

	if N <= 1 {
		stdDev = 0.0
	} else {
		varianceSum := 0.0
		for _, v := range ds.data {
			varianceSum += math.Pow(v-avg, 2)
		}
		stdDev = math.Sqrt(varianceSum / (N - 1))
	}
	ds.stdDev = stdDev
	return avg, stdDev
}

func (ds *DeviceStats) CalculateZScore(value float64, avg float64, stdDev float64) float64 {
	if stdDev == 0.0 || len(ds.data) < 2 {
		return 0.0
	}
	z := (value - avg) / stdDev
	ds.LastZScore = z
	return z
}

type AnalyticsResult struct {
	DeviceID string
	Value    float64
}

type AnomalyDetector struct {
	DeviceStatsMap map[string]*DeviceStats
	ResultChan     chan AnalyticsResult
	Threshold      float64
	mu             sync.RWMutex
}

func NewAnomalyDetector(capacity int, threshold float64) *AnomalyDetector {
	return &AnomalyDetector{
		DeviceStatsMap: make(map[string]*DeviceStats),
		ResultChan:     make(chan AnalyticsResult, 100),
		Threshold:      threshold,
	}
}

func (ad *AnomalyDetector) GetOrCreateStats(deviceID string, capacity int) *DeviceStats {
	ad.mu.RLock()
	stats, exists := ad.DeviceStatsMap[deviceID]
	ad.mu.RUnlock()

	if exists {
		return stats
	}

	ad.mu.Lock()
	defer ad.mu.Unlock()
	if stats, exists := ad.DeviceStatsMap[deviceID]; exists {
		return stats
	}

	newStats := NewDeviceStats(capacity)
	ad.DeviceStatsMap[deviceID] = newStats
	return newStats
}

func (ad *AnomalyDetector) ProcessData(deviceID string, value float64) {
	stats := ad.GetOrCreateStats(deviceID, AvgWindowSize)

	avg, stdDev := stats.AddAndCalculate(value)
	zScore := stats.CalculateZScore(value, avg, stdDev)
	isAnomaly := math.Abs(zScore) > ad.Threshold

	if isAnomaly {
		//log.Printf("[ANOMALY] Device: %s, Value: %.2f, Z-Score: %.3f", deviceID, value, zScore)
		Metrics.AnomalyTotalCounter.WithLabelValues(deviceID).Inc()
		stats.mu.Lock()
		stats.AnomalyCountTotal++
		stats.mu.Unlock()
	}

	ad.ResultChan <- AnalyticsResult{
		DeviceID: deviceID,
		Value:    value,
	}
}
