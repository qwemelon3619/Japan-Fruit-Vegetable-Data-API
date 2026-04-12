package monitoring

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type httpRequestKey struct {
	Method      string
	Path        string
	StatusClass string
}

type httpLatencyKey struct {
	Method string
	Path   string
}

type metricsStore struct {
	mu sync.Mutex

	httpRequests map[httpRequestKey]uint64
	httpLatency  map[httpLatencyKey]*histogram
	dbLatency    map[string]*histogram
	dbErrors     map[string]uint64
	inflight     int64
}

func newMetricsStore() *metricsStore {
	return &metricsStore{
		httpRequests: make(map[httpRequestKey]uint64, 64),
		httpLatency:  make(map[httpLatencyKey]*histogram, 64),
		dbLatency:    make(map[string]*histogram, 64),
		dbErrors:     make(map[string]uint64, 64),
	}
}

func (m *metricsStore) incInflight() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inflight++
}

func (m *metricsStore) decInflight() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inflight--
	if m.inflight < 0 {
		m.inflight = 0
	}
}

func (m *metricsStore) observeHTTP(method, path string, status int, seconds float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rKey := httpRequestKey{Method: method, Path: path, StatusClass: statusClass(status)}
	m.httpRequests[rKey]++

	lKey := httpLatencyKey{Method: method, Path: path}
	h := m.httpLatency[lKey]
	if h == nil {
		h = newHistogram()
		m.httpLatency[lKey] = h
	}
	h.observe(seconds)
}

func (m *metricsStore) observeDB(query string, seconds float64, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h := m.dbLatency[query]
	if h == nil {
		h = newHistogram()
		m.dbLatency[query] = h
	}
	h.observe(seconds)
	if err != nil {
		m.dbErrors[query]++
	}
}

func (m *metricsStore) renderPrometheus() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var sb strings.Builder

	sb.WriteString("# HELP http_requests_total Total number of HTTP requests\n")
	sb.WriteString("# TYPE http_requests_total counter\n")
	hReqKeys := make([]httpRequestKey, 0, len(m.httpRequests))
	for k := range m.httpRequests {
		hReqKeys = append(hReqKeys, k)
	}
	sort.Slice(hReqKeys, func(i, j int) bool {
		if hReqKeys[i].Path != hReqKeys[j].Path {
			return hReqKeys[i].Path < hReqKeys[j].Path
		}
		if hReqKeys[i].Method != hReqKeys[j].Method {
			return hReqKeys[i].Method < hReqKeys[j].Method
		}
		return hReqKeys[i].StatusClass < hReqKeys[j].StatusClass
	})
	for _, k := range hReqKeys {
		sb.WriteString(fmt.Sprintf("http_requests_total{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n",
			escapeLabel(k.Method), escapeLabel(k.Path), escapeLabel(k.StatusClass), m.httpRequests[k]))
	}

	sb.WriteString("# HELP http_inflight_requests In-flight HTTP requests\n")
	sb.WriteString("# TYPE http_inflight_requests gauge\n")
	sb.WriteString(fmt.Sprintf("http_inflight_requests %d\n", m.inflight))

	sb.WriteString("# HELP http_request_duration_seconds HTTP request duration in seconds\n")
	sb.WriteString("# TYPE http_request_duration_seconds histogram\n")
	hLatKeys := make([]httpLatencyKey, 0, len(m.httpLatency))
	for k := range m.httpLatency {
		hLatKeys = append(hLatKeys, k)
	}
	sort.Slice(hLatKeys, func(i, j int) bool {
		if hLatKeys[i].Path != hLatKeys[j].Path {
			return hLatKeys[i].Path < hLatKeys[j].Path
		}
		return hLatKeys[i].Method < hLatKeys[j].Method
	})
	for _, k := range hLatKeys {
		m.httpLatency[k].render(&sb, "http_request_duration_seconds", map[string]string{
			"method": k.Method,
			"path":   k.Path,
		})
	}

	sb.WriteString("# HELP db_query_duration_seconds Database query duration in seconds\n")
	sb.WriteString("# TYPE db_query_duration_seconds histogram\n")
	dbKeys := make([]string, 0, len(m.dbLatency))
	for k := range m.dbLatency {
		dbKeys = append(dbKeys, k)
	}
	sort.Strings(dbKeys)
	for _, k := range dbKeys {
		m.dbLatency[k].render(&sb, "db_query_duration_seconds", map[string]string{"query": k})
	}

	sb.WriteString("# HELP db_errors_total Total number of DB query errors\n")
	sb.WriteString("# TYPE db_errors_total counter\n")
	errKeys := make([]string, 0, len(m.dbErrors))
	for k := range m.dbErrors {
		errKeys = append(errKeys, k)
	}
	sort.Strings(errKeys)
	for _, k := range errKeys {
		sb.WriteString(fmt.Sprintf("db_errors_total{query=\"%s\"} %d\n", escapeLabel(k), m.dbErrors[k]))
	}

	return sb.String()
}

type histogram struct {
	buckets []float64
	counts  []uint64
	count   uint64
	sum     float64
}

func newHistogram() *histogram {
	b := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	return &histogram{buckets: b, counts: make([]uint64, len(b))}
}

func (h *histogram) observe(v float64) {
	h.count++
	h.sum += v
	for i, b := range h.buckets {
		if v <= b {
			h.counts[i]++
		}
	}
}

func (h *histogram) render(sb *strings.Builder, metric string, labels map[string]string) {
	for i, b := range h.buckets {
		withLe := copyLabels(labels)
		withLe["le"] = fmt.Sprintf("%g", b)
		sb.WriteString(fmt.Sprintf("%s_bucket{%s} %d\n", metric, renderLabels(withLe), h.counts[i]))
	}
	inf := copyLabels(labels)
	inf["le"] = "+Inf"
	sb.WriteString(fmt.Sprintf("%s_bucket{%s} %d\n", metric, renderLabels(inf), h.count))
	sb.WriteString(fmt.Sprintf("%s_sum{%s} %g\n", metric, renderLabels(labels), h.sum))
	sb.WriteString(fmt.Sprintf("%s_count{%s} %d\n", metric, renderLabels(labels), h.count))
}

func copyLabels(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func renderLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", k, escapeLabel(labels[k])))
	}
	return strings.Join(parts, ",")
}

func escapeLabel(v string) string {
	v = strings.ReplaceAll(v, "\\", "\\\\")
	v = strings.ReplaceAll(v, "\"", "\\\"")
	v = strings.ReplaceAll(v, "\n", "\\n")
	return v
}

func statusClass(status int) string {
	switch {
	case status >= 100 && status < 200:
		return "1xx"
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "other"
	}
}
