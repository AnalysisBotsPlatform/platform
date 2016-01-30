package utils

import (
	"math/rand"
	"time"
)

// List of characters used to generate random character sequence.
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"0123456789"
const (
	letterIdxBits = 6
	letterIdxMask = 1<<letterIdxBits - 1
	letterIdxMax  = 63 / letterIdxBits
)

// Random number generator.
var src = rand.NewSource(time.Now().UnixNano())

// Generates a sequence of random characters (`letterBytes`) of length `n`.
func RandString(n int) string {
	b := make([]byte, n)

	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func ComputeDate(t time.Time, day int) time.Time{
    
    currentDay := int(t.Weekday())
    var dayDiff int
    if(day >= currentDay){
        dayDiff = day - currentDay
    }else{
        dayDiff = 7 - (currentDay - day)
    }
    
    return t.AddDate(0,0, dayDiff)
    
}
