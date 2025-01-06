package statsig

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func defaultString(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func getHash(key string) []byte {
	hasher := sha256.New()
	bytes := []byte(key)
	hasher.Write(bytes)
	return hasher.Sum(nil)
}

func getDJB2Hash(key string) string {
	hash := uint64(0)
	bytes := []byte(key)
	for _, b := range bytes {
		hash = ((hash << 5) - hash) + uint64(b)
		hash = hash & ((1 << 32) - 1)
	}
	return strconv.FormatUint(hash, 10)
}

func getHashUint64Encoding(key string) uint64 {
	hash := getHash(key)
	return binary.BigEndian.Uint64(hash)
}

func getHashBase64StringEncoding(configName string) string {
	hash := getHash(configName)
	return base64.StdEncoding.EncodeToString(hash)
}

func safeGetFirst(slice []string) string {
	if len(slice) > 0 {
		return slice[0]
	}
	return ""
}

func safeParseJSONint64(val interface{}) int64 {
	if num, ok := val.(json.Number); ok {
		i64, _ := strconv.ParseInt(string(num), 10, 64)
		return i64
	} else {
		return 0
	}
}

func compareMetadata(t *testing.T, metadata map[string]string, expected map[string]string, time int64) {
	v, _ := json.Marshal(metadata)
	var rawMetadata map[string]string
	_ = json.Unmarshal(v, &rawMetadata)

	for key, value1 := range expected {
		if value2, exists := metadata[key]; exists {
			if value1 != value2 {
				t.Errorf("Values for key '%s' do not match. Expected: %+v. Received: %+v", key, value1, value2)
			}
		} else {
			t.Errorf("Key '%s' does not exist in metadata", key)
		}
	}

	for _, key := range []string{"configSyncTime", "initTime"} {
		value, exists := metadata[key]
		if !exists {
			t.Errorf("'%s' does not exist in metadata", key)
		}

		if strconv.FormatInt(time, 10) != value {
			t.Errorf("'%s' does not have the expected time %d. Actual %s", key, time, value)
		}
	}

	now := getUnixMilli()
	serverTime, _ := strconv.ParseInt(metadata["serverTime"], 10, 64)
	if now-1000 <= serverTime && serverTime <= now+1000 {
		return
	}

	t.Errorf("serverTime is outside of the valid range. Expected %d (±2000), Actual %d", now, serverTime)
}

func toError(err interface{}) error {
	errAsError, ok := err.(error)
	if ok {
		return errAsError
	} else {
		errAsString, ok := err.(string)
		if ok {
			return errors.New(errAsString)
		} else {
			return errors.New("")
		}
	}
}

func getUnixMilli() int64 {
	// time.Now().UnixMilli() wasn't added until go 1.17
	unixNano := time.Now().UnixNano()
	return unixNano / int64(time.Millisecond)
}

func hashName(hashAlgorithm string, name string) string {
	switch hashAlgorithm {
	case "sha256":
		return getHashBase64StringEncoding(name)
	case "djb2":
		return getDJB2Hash(name)
	default:
		return name
	}
}

func getNumericValue(a interface{}) (float64, bool) {
	if a == nil {
		return 0, false
	}
	aVal := reflect.ValueOf(a)
	switch reflect.TypeOf(a).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(aVal.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(aVal.Uint()), true
	case reflect.Float32, reflect.Float64:
		return float64(aVal.Float()), true
	case reflect.String:
		f, err := strconv.ParseFloat(aVal.String(), 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}
