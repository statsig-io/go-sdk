package statsig

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStoreSync(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		if strings.Contains(req.URL.Path, "download_config_specs") {
			var in *downloadConfigsInput
			json.NewDecoder(req.Body).Decode(&in)
			r := &downloadConfigSpecResponse{
				HasUpdates:     true,
				Time:           time.Now().Unix(),
				FeatureGates:   []configSpec{{Name: "gate_1"}},
				DynamicConfigs: []configSpec{{Name: "exp_1"}},
				IDLists:        map[string]bool{"list_1": true},
			}
			if in.SinceTime > 0 {
				r = &downloadConfigSpecResponse{
					HasUpdates:     true,
					Time:           time.Now().Unix(),
					FeatureGates:   []configSpec{{Name: "gate_1"}, {Name: "gate_2"}},
					DynamicConfigs: []configSpec{{Name: "exp_1"}, {Name: "exp_2"}},
					IDLists:        map[string]bool{"list_1": true, "list_2": true},
				}
			}
			v, _ := json.Marshal(r)
			res.Write(v)
		} else if strings.Contains(req.URL.Path, "download_id_list") {
			var in *downloadIDListInput
			json.NewDecoder(req.Body).Decode(&in)

			r := &downloadIDListResponse{
				AddIDs:    []string{"1", "2", "3"},
				RemoveIDs: []string{},
				Time:      time.Now().Unix(),
			}
			if in.SinceTime > 0 {
				r = &downloadIDListResponse{
					AddIDs:    []string{"4", "5"},
					RemoveIDs: []string{"1", "2"},
					Time:      time.Now().Unix(),
				}
			}
			v, _ := json.Marshal(r)
			res.Write(v)
		}
	}))
	defer testServer.Close()
	opt := &Options{
		API: testServer.URL,
	}
	n := newTransport("secret-123", opt)
	s := newStoreInternal(n, time.Second, time.Second)

	time.Sleep(time.Second)
	if len(s.featureGates) != 1 {
		t.Errorf("Wrong number of feature gates after 1 sec")
	}
	if len(s.dynamicConfigs) != 1 {
		t.Errorf("Wrong number of configs after 1 sec")
	}
	if len(s.idLists) != 1 {
		t.Errorf("Wrong number of id lists after 1 sec")
	}

	firstList := map[string]bool{"1": true, "2": true, "3": true}
	secondList := map[string]bool{"3": true, "4": true, "5": true}

	// after 1 sec, list_1 should have 1,2,3 and list_2 should not exist yet
	if !reflect.DeepEqual(s.idLists["list_1"].ids, firstList) {
		t.Errorf("list_1 incorrect after 1 sec")
	}

	if _, ok := s.idLists["list_2"]; ok {
		t.Errorf("list_2 should not exist after 1 sec")
	}

	time.Sleep(time.Second)
	// after 2 sec, list_1 should have 3,4,5 and list_2 should be empty
	if !reflect.DeepEqual(s.idLists["list_1"].ids, secondList) {
		t.Errorf("list_1 incorrect after 2 sec")
	}
	if len(s.idLists["list_2"].ids) != 0 || s.idLists["list_2"].time != int64(0) {
		t.Errorf("list_2 incorrect after 2 sec")
	}

	time.Sleep(time.Second)
	// after 3 sec, list_1 should have 3,4,5 and list_2 should have 1,2,3
	if !reflect.DeepEqual(s.idLists["list_1"].ids, secondList) {
		t.Errorf("list_1 incorrect after 3 sec")
	}
	if !reflect.DeepEqual(s.idLists["list_2"].ids, firstList) {
		t.Errorf("list_2 incorrect after 3 sec")
	}

	time.Sleep(time.Second)
	// after 4 sec, both lists should be updated to 3,4,5
	if !reflect.DeepEqual(s.idLists["list_1"].ids, secondList) {
		t.Errorf("list_1 incorrect after 4 sec")
	}
	if !reflect.DeepEqual(s.idLists["list_2"].ids, secondList) {
		t.Errorf("list_2 incorrect after 4 sec")
	}
}
