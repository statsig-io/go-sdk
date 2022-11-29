package statsig

import "time"

func defaultString(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

// Allows for overriding in tests
var now = time.Now
