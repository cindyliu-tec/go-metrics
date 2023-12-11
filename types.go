package go_metrics

import (
	"net/http"
	"time"

	"git.makeblock.com/makeblock-go/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
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
	defaultDuration = []float64{0.05, 0.1, 0.2, 0.3, 0.4, 0.6, 0.8, 1, 2, 3, 5, 10}
	initLabels      = []string{"env", "namespace", "app", "pod"}
	monitor         *Monitor

	promTypeHandler = map[MetricType]func(metric *Metric) error{
		Counter:   counterHandler,
		Gauge:     gaugeHandler,
		Histogram: histogramHandler,
		Summary:   summaryHandler,
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
			promRegistry: prometheus.NewRegistry(),
		}
	}
	return monitor
}

// 启动指标服务端口
func (m *Monitor) setupServer() error {
	http.Handle(m.metricPath, promhttp.HandlerFor(m.promRegistry, promhttp.HandlerOpts{}))
	var err error
	go func() {
		log.Println("Start Metric Server ", m.server.Addr)
		if err = m.server.ListenAndServe(); err != nil {
			log.ErrorE("Failed to serve: ", err)
		}
	}()
	return err
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
func (m *Monitor) SetDuration(duration []float64) {
	m.reqDuration = duration
}

// AddMetric 添加自定义指标.
func (m *Monitor) AddMetric(metric *Metric) error {
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
		}
	}
	return errors.Errorf("metric type '%d' not existed.", metric.Type)
}

//nolint unparam
func counterHandler(metric *Metric) error {
	metric.vec = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: metric.Name, Help: metric.Description},
		metric.Labels,
	)
	return nil
}

//nolint unparam
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
