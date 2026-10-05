package metrics

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"testing"
)

// dashboard is the example Grafana dashboard (WB-17), relative to this package.
const dashboard = "../../examples/grafana/whipbench-dashboard.json"

// jsonStrings returns every string value in a decoded JSON document.
func jsonStrings(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, e := range v {
			out = append(out, jsonStrings(e)...)
		}
		return out
	case map[string]any:
		var out []string
		for _, e := range v {
			out = append(out, jsonStrings(e)...)
		}
		return out
	}
	return nil
}

// Every metric name the dashboard queries is one /metrics serves, by exact name: a
// family, or a histogram family's _bucket, _sum or _count.
func TestDashboardQueriesServedSeries(t *testing.T) {
	raw, err := os.ReadFile(dashboard)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s is not JSON: %v", dashboard, err)
	}
	var b bytes.Buffer
	(&Live{}).Write(&b)
	types, err := families(b.String())
	if err != nil {
		t.Fatal(err)
	}
	served := map[string]bool{}
	for name, typ := range types {
		served[name] = true
		if typ == "histogram" {
			served[name+"_bucket"], served[name+"_sum"], served[name+"_count"] = true, true, true
		}
	}
	name := regexp.MustCompile(`\bwhipbench_[a-zA-Z0-9_]+`)
	queried := map[string]bool{}
	for _, s := range jsonStrings(doc) {
		for _, n := range name.FindAllString(s, -1) {
			queried[n] = true
			if !served[n] {
				t.Errorf("the dashboard queries %s, which /metrics does not serve", n)
			}
		}
	}
	for _, want := range []string{"whipbench_viewers_active", "whipbench_viewers_failed_total", "whipbench_rtp_packets_received_total",
		"whipbench_one_way_delay_seconds_bucket", "whipbench_packet_transit_seconds_bucket",
		"whipbench_client_cpu_seconds_total", "whipbench_client_goroutines", "whipbench_client_heap_bytes"} {
		if !queried[want] {
			t.Errorf("the dashboard has no panel over %s", want)
		}
	}
}

// The data source is a variable, so the dashboard imports into any Grafana with a
// Prometheus that scrapes the run, and every panel and query uses it.
func TestDashboardDataSourceIsAVariable(t *testing.T) {
	raw, err := os.ReadFile(dashboard)
	if err != nil {
		t.Fatal(err)
	}
	type ds struct{ Type, UID string }
	var d struct {
		SchemaVersion int `json:"schemaVersion"`
		Templating    struct {
			List []struct{ Name, Type, Query any }
		}
		Panels []struct {
			Title      string
			Datasource ds
			Targets    []struct {
				Datasource ds
				Expr       string
			}
		}
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if d.SchemaVersion < 36 {
		t.Errorf("schemaVersion %d predates Grafana 10's panel datasource references", d.SchemaVersion)
	}
	if !slices.ContainsFunc(d.Templating.List, func(v struct{ Name, Type, Query any }) bool {
		return v.Name == "datasource" && v.Type == "datasource" && v.Query == "prometheus"
	}) {
		t.Error("no Prometheus datasource variable named datasource")
	}
	want := ds{Type: "prometheus", UID: "${datasource}"}
	for _, p := range d.Panels {
		if p.Datasource != want || len(p.Targets) == 0 {
			t.Errorf("panel %q: datasource %+v, %d queries", p.Title, p.Datasource, len(p.Targets))
		}
		for _, q := range p.Targets {
			if q.Datasource != want || q.Expr == "" {
				t.Errorf("panel %q: query %q on %+v", p.Title, q.Expr, q.Datasource)
			}
		}
	}
}
