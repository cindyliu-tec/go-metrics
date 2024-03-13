package go_metrics

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.makeblock.com/makeblock-go/log"
	"github.com/bytedance/sonic"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricType int

const (
	None MetricType = iota
	Counter
	Gauge
	Histogram
	Summary

	defaultMetricPath = "/mk/metrics"
	defaultPort       = "13116"
	defaultSlowTime   = int32(5)
	timeoutInSeconds  = 5
	metricPrefix      = "mk_"
)

var (
	defaultDuration               = []float64{0.005, 0.02, 0.05, 0.1, 0.15, 0.2, 0.3, 0.5, 1, 2, 5}
	customMetricDefaultDuration   = []float64{5, 20, 50, 100, 200, 300, 500, 1000, 5000}
	customMetricDefaultObjectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}
	initLabels                    = []string{"env", "namespace", "app", "pod"}
	monitor                       *Monitor

	promTypeHandler = map[MetricType]func(metric *Metric) error{
		Counter:   counterHandler,
		Gauge:     gaugeHandler,
		Histogram: histogramHandler,
		Summary:   summaryHandler,
	}

	// 响应对象池
	responseWriterPool = sync.Pool{
		New: func() any {
			return &responseWriter{
				code: defaultBusinessCode,
			}
		},
	}
)

// Monitor is an object that uses to set gin server monitor.
type Monitor struct {
	slowTime     int32
	metricPath   string
	reqDuration  []float64
	metrics      map[string]*Metric
	server       *http.Server
	promRegistry *prometheus.Registry
}

// GetMonitor 返回单例 Monitor 对象
func GetMonitor() *Monitor {
	if monitor == nil {
		registry := prometheus.NewRegistry()
		if err := registry.Register(collectors.NewGoCollector()); err != nil {
			log.ErrorE("register go runtime metrics failed", err)
		}
		monitor = &Monitor{
			metricPath:  defaultMetricPath,
			slowTime:    defaultSlowTime,
			reqDuration: defaultDuration,
			metrics:     make(map[string]*Metric),
			server: &http.Server{
				Addr:              ":" + defaultPort,
				ReadTimeout:       timeoutInSeconds * time.Second,
				ReadHeaderTimeout: timeoutInSeconds * time.Second,
				WriteTimeout:      timeoutInSeconds * time.Second,
			},
			promRegistry: registry,
		}
	}
	return monitor
}

// 启动指标服务端口
func (m *Monitor) setupServer() {
	http.Handle(m.metricPath, promhttp.HandlerFor(m.promRegistry, promhttp.HandlerOpts{}))
	go func() {
		log.Println("Start Metric Server ", m.server.Addr)
		if err := m.server.ListenAndServe(); err != nil {
			log.ErrorE("Failed to serve: ", err)
		}
	}()
}

// GetMetric used to get metric object by metric_name.
func (m *Monitor) GetMetric(name string) *Metric {
	if metric, ok := m.metrics[metricPrefix+name]; ok {
		return metric
	}
	return &Metric{}
}

// SetMetricPath set metricPath property. metricPath is used for Prometheus
// to get gin server monitoring data.
func (m *Monitor) SetMetricPath(path string) {
	m.metricPath = path
}

func (m *Monitor) SetPort(port string) {
	m.server.Addr = ":" + port
}

// SetSlowTime set slowTime property. slowTime is used to determine whether
// the request is slow. For "gin_slow_request_total" metric.
func (m *Monitor) SetSlowTime(slowTime int32) {
	m.slowTime = slowTime
}

// SetDuration set reqDuration property. reqDuration is used to ginRequestDuration
// metric buckets.
func (m *Monitor) setDuration(duration []float64) {
	m.reqDuration = duration
}

// AddMetric 添加指标.
func (m *Monitor) addMetric(metric *Metric) error {
	if metric.Name == "" {
		return errors.Errorf("metric name cannot be empty.")
	}
	metric.Name = metricPrefix + metric.Name
	if _, ok := m.metrics[metric.Name]; ok {
		return errors.Errorf("metric '%s' is existed", metric.Name)
	}

	if f, ok := promTypeHandler[metric.Type]; ok {
		metric.Labels = append(initLabels, metric.Labels...)
		if err := f(metric); err == nil {
			m.promRegistry.MustRegister(metric.vec)
			m.metrics[metric.Name] = metric
			return nil
		} else {
			return errors.Wrap(err, "register metric failed")
		}
	}
	return errors.Errorf("metric type '%d' not existed.", metric.Type)
}

// AddCustomMetric 注册自定义指标.
func (m *Monitor) AddCustomMetric(metric *Metric) error {
	if metric.Name == "" {
		return errors.Errorf("metric name cannot be empty.")
	}
	metric.Name = metricPrefix + metric.Name
	if _, ok := m.metrics[metric.Name]; ok {
		return errors.Errorf("metric '%s' is existed", metric.Name)
	}

	if metric.Type == Histogram && metric.Buckets == nil {
		metric.Buckets = customMetricDefaultDuration
	}

	if metric.Type == Summary && metric.Objectives == nil {
		metric.Objectives = customMetricDefaultObjectives
	}

	if f, ok := promTypeHandler[metric.Type]; ok {
		metric.Labels = append(initLabels, metric.Labels...)
		if err := f(metric); err == nil {
			m.promRegistry.MustRegister(metric.vec)
			m.metrics[metric.Name] = metric
			metricType := strconv.Itoa(int(metric.Type))
			labelNames := strings.Join(metric.Labels, ",")
			_ = m.GetMetric(metricCustom).SetGaugeValue([]string{metric.Name, metricType, metric.Description, labelNames}, 1)
			return nil
		} else {
			return errors.Wrap(err, "register custom metric failed")
		}
	}
	return errors.Errorf("metric type '%d' not existed.", metric.Type)
}

// nolint unparam
func counterHandler(metric *Metric) error {
	metric.vec = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: metric.Name, Help: metric.Description},
		metric.Labels,
	)
	return nil
}

// nolint unparam
func gaugeHandler(metric *Metric) error {
	metric.vec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: metric.Name, Help: metric.Description},
		metric.Labels,
	)
	return nil
}

func histogramHandler(metric *Metric) error {
	if len(metric.Buckets) == 0 {
		return errors.Errorf("metric '%s' is histogram type, cannot lose bucket param.", metric.Name)
	}
	metric.vec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: metric.Name, Help: metric.Description, Buckets: metric.Buckets},
		metric.Labels,
	)
	return nil
}

func summaryHandler(metric *Metric) error {
	if len(metric.Objectives) == 0 {
		return errors.Errorf("metric '%s' is summary type, cannot lose objectives param.", metric.Name)
	}
	prometheus.NewSummaryVec(
		prometheus.SummaryOpts{Name: metric.Name, Help: metric.Description, Objectives: metric.Objectives},
		metric.Labels,
	)
	return nil
}

type responseWriter struct {
	gin.ResponseWriter
	code string
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if jsonCTExpr.MatchString(w.Header().Get("content-type")) {
		if data, err := sonic.Get(b, "code"); err == nil {
			if code, err := data.Raw(); err != nil {
				w.code = "-2"
			} else {
				w.code = code
			}
		}
	}
	return w.ResponseWriter.Write(b)
}
