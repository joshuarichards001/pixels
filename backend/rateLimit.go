package main

import "time"

func (server *Server) checkRateLimit(ip string) bool {
	now := time.Now()

	ipRateLimitData, _ := server.rateLimits.LoadOrStore(ip, &RateLimitData{})
	ipPixelUpdateTimes := ipRateLimitData.(*RateLimitData)

	ipPixelUpdateTimes.mu.Lock()
	defer ipPixelUpdateTimes.mu.Unlock()

	cutoff := now.Add(-5 * time.Second)
	updatesInTimeLimit := []time.Time{}
	for _, updateTime := range ipPixelUpdateTimes.timestamps {
		if updateTime.After(cutoff) {
			updatesInTimeLimit = append(updatesInTimeLimit, updateTime)
		}
	}

	if len(updatesInTimeLimit) >= 20 {
		ipPixelUpdateTimes.timestamps = updatesInTimeLimit
		return false
	}

	updatesInTimeLimit = append(updatesInTimeLimit, now)
	ipPixelUpdateTimes.timestamps = updatesInTimeLimit

	if now.Unix()%60 == 0 {
		go server.cleanupRateLimits()
	}

	return true
}

func (server *Server) cleanupRateLimits() {
	now := time.Now()
	cutoff := now.Add(-10 * time.Minute)

	server.rateLimits.Range(func(key, value interface{}) bool {
		ipPixelUpdateTimes := value.(*RateLimitData)
		ipPixelUpdateTimes.mu.Lock()
		defer ipPixelUpdateTimes.mu.Unlock()

		if len(ipPixelUpdateTimes.timestamps) == 0 || ipPixelUpdateTimes.timestamps[len(ipPixelUpdateTimes.timestamps)-1].Before(cutoff) {
			server.rateLimits.Delete(key)
		}
		
		return true
	})
}
