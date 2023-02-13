package statsig

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"
)

func defaultString(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

// Allows for overriding in tests
var now = time.Now

func getHash(key string) []byte {
	hasher := sha256.New()
	bytes := []byte(key)
	hasher.Write(bytes)
	return hasher.Sum(nil)
}

func getHashUint64Encoding(key string) uint64 {
	hash := getHash(key)
	return binary.BigEndian.Uint64(hash)
}

func getHashBase64StringEncoding(configName string) string {
	hash := getHash(configName)
	return base64.StdEncoding.EncodeToString(hash)
}

func logProcessWithTimestamp(process string, msg string) {
	timestamp := now().Format(time.RFC3339)
	fmt.Printf("[%s][Statsig] %s: %s\n", timestamp, process, msg)
}
