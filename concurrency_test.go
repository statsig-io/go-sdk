package statsig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCallingAPIsConcurrently(t *testing.T) {
	flushedEventCount := int32(0)
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			var in *downloadConfigsInput
			bytes, _ := ioutil.ReadFile("download_config_specs.json")
			json.NewDecoder(req.Body).Decode(&in)
			res.Write(bytes)
		} else if strings.Contains(req.URL.Path, "log_event") {
			type requestInput struct {
				Events          []Event         `json:"events"`
				StatsigMetadata statsigMetadata `json:"statsigMetadata"`
			}
			input := &requestInput{}
			defer req.Body.Close()
			buf := new(bytes.Buffer)
			buf.ReadFrom(req.Body)

			json.Unmarshal(buf.Bytes(), &input)
			atomic.AddInt32((&flushedEventCount), int32(len(input.Events)))
		} else if strings.Contains(req.URL.Path, "get_id_lists") {
			baseURL := "http://" + req.Host
			r := map[string]idList{
				"list_1": {Name: "list_1", Size: 3, URL: baseURL + "/list_1", CreationTime: 1, FileID: "file_id_1"},
				"list_2": {Name: "list_2", Size: 3, URL: baseURL + "/list_2", CreationTime: 1, FileID: "file_id_2"},
			}
			v, _ := json.Marshal(r)
			res.Write(v)
		} else if strings.Contains(req.URL.Path, "list_1") {
			res.Write([]byte("+7/rrkvF6\n"))
		}
	}))

	defer testServer.Close()
	options := &Options{
		API: testServer.URL,
		Environment: Environment{
			Params: map[string]string{
				"foo": "bar",
			},
			Tier: "awesome_land",
		},
	}

	InitializeWithOptions("secret-key", options)

	const (
		goroutines = 10
		loops      = 10
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			statsigUser := User{
				UserID:            fmt.Sprintf("statsig_u_%d", g),
				Email:             "u@statsig.com",
				PrivateAttributes: map[string]interface{}{"private": "shh"},
				Custom:            map[string]interface{}{"key": "value"},
				CustomIDs:         map[string]string{"randomID": "123456"},
			}
			user := User{
				UserID: "regular_user_id",
				Email:  "u@gmail.com",
			}
			for i := 0; i < loops; i++ {
				LogEvent(Event{EventName: "test_event", User: user})
				if CheckGate(statsigUser, "on_for_id_list") || !CheckGate(user, "on_for_id_list") {
					t.Error("statsigUser should fail and regular user should pass for on_for_id_list gate")
				}

				if !CheckGate(statsigUser, "on_for_statsig_email") || CheckGate(user, "on_for_statsig_email") {
					t.Error("statsig user should pass statsig email gate and regular user should fail")
				}
				LogEvent(Event{EventName: "test_event_2", User: statsigUser})
				exp := GetExperiment(statsigUser, "sample_experiment")
				if !exp.GetBool("layer_param", false) {
					t.Error("sample_experiment layer_param not correct")
				}
				config := GetConfig(statsigUser, "test_config")
				if config.GetNumber("number", 420) != 7 {
					t.Error("test_config number not correct")
				}
				LogEvent(Event{EventName: "test_event_3", User: statsigUser})
				layer := GetLayer(statsigUser, "a_layer")
				if !layer.GetBool("layer_param", false) {
					t.Error("sample_experiment layer_param not correct")
				}
				LogEvent(Event{EventName: "test_event_4", User: statsigUser})
			}
		}()
	}
	wg.Wait()

	// 10 go routines x 10 loops each x 9 events (4 log event + 7 exposure events) = 1100 total events should have been logged.

	// only 100 should still be in the logger now because the first 1000 would have been cut and triggered a flush
	if len(instance.logger.events) != 100 {
		t.Error("Incorrect number of events batched in the logger")
	}

	Shutdown()

	// wait a little to allow the async flush to be executed
	time.Sleep(time.Second)
	if atomic.LoadInt32(&flushedEventCount) != 1100 {
		t.Error("Not all events were flushed eventually")
	}
}

func TestUpdatingRulesAndFetchingValuesConcurrently(t *testing.T) {
	configSyncCount := 0
	idlistSyncCount := 0
	idListContent := "+7/rrkvF6\n" // hashed value for "regular_user_id", which is used for a user below
	idListSize := len(idListContent)

	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			configSyncCount++
			var in *downloadConfigsInput
			bytes, _ := ioutil.ReadFile("download_config_specs.json")
			json.NewDecoder(req.Body).Decode(&in)
			res.Write(bytes)
		} else if strings.Contains(req.URL.Path, "get_id_lists") {
			idlistSyncCount++
			baseURL := "http://" + req.Host
			r := map[string]idList{
				"list_1": {Name: "list_1", Size: int64(idListSize), URL: baseURL + "/list_1", CreationTime: 3, FileID: "file_id_1"},
			}

			v, _ := json.Marshal(r)
			res.Write(v)
		} else if strings.Contains(req.URL.Path, "list_1") {
			res.Write([]byte(idListContent))
			idListContent = fmt.Sprintf("+%d\n-%d\n", idlistSyncCount, idlistSyncCount)
			idListSize += len(idListContent)
		}
	}))

	defer testServer.Close()
	options := &Options{
		API: testServer.URL,
		Environment: Environment{
			Params: map[string]string{
				"foo": "bar",
			},
			Tier: "awesome_land",
		},
		// override sync interval so the rulesets and idlists get updated rapidly
		ConfigSyncInterval: time.Millisecond * 10,
		IDListSyncInterval: time.Millisecond * 10,
	}

	client := NewClientWithOptions("secret-Key", options)

	const (
		goroutines = 10
		duration   = time.Second * 3
	)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := time.Now()
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			statsigUser := User{
				UserID:            fmt.Sprintf("statsig_u_%d", g),
				Email:             "u@statsig.com",
				PrivateAttributes: map[string]interface{}{"private": "shh"},
				Custom:            map[string]interface{}{"key": "value"},
				CustomIDs:         map[string]string{"randomID": "123456"},
			}
			user := User{
				UserID: "regular_user_id",
				Email:  "u@gmail.com",
			}
			for time.Since(start) < duration {
				// checking for the id list that's getting updated constantly
				if client.CheckGate(statsigUser, "on_for_id_list") || !client.CheckGate(user, "on_for_id_list") {
					t.Error("statsigUser should fail and regular user should pass for on_for_id_list gate")
				}

				if !client.CheckGate(statsigUser, "on_for_statsig_email") || client.CheckGate(user, "on_for_statsig_email") {
					t.Error("statsig user should pass statsig email gate and regular user should fail")
				}
			}
		}()
	}
	wg.Wait()
	client.Shutdown()
}

func TestOverrideAPIsConcurrency(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			var in *downloadConfigsInput
			bytes, _ := ioutil.ReadFile("download_config_specs.json")
			json.NewDecoder(req.Body).Decode(&in)
			res.Write(bytes)
		}
	}))

	defer testServer.Close()
	options := &Options{
		API: testServer.URL,
		Environment: Environment{
			Params: map[string]string{
				"foo": "bar",
			},
			Tier: "awesome_land",
		},
	}

	client := NewClientWithOptions("secret-Key", options)

	const (
		goroutines = 10
		duration   = time.Second * 3
	)
	user := User{
		UserID: "regular_user_id",
		Email:  "u@gmail.com",
	}
	var wg sync.WaitGroup
	wg.Add(goroutines)
	start := time.Now()
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for time.Since(start) < duration {
				client.OverrideGate("always_on_gate", true)
				client.OverrideGate("always_on_gate", false)
				client.OverrideConfig("test_config", map[string]interface{}{"v": "123"})
			}
		}()
	}
	wg.Wait()

	if client.CheckGate(user, "always_on_gate") {
		t.Error("gate should have been overridden to off")
	}
	config := client.GetConfig(user, "test_config")
	if config.GetString("v", "str") != "123" {
		t.Error("config should have been overridden to have 123")
	}

	client.Shutdown()
}
