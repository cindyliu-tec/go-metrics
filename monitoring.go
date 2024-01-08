package go_metrics

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	metricRequestDuration = "gin_uri_request_duration"
	metricSlowRequest     = "gin_slow_request_total"
	metricLongRequest     = "gin_long_request_total"
	metricSDKVersion      = "monitor_sdk_version"
	jsonCTExpr, _         = regexp.Compile("application/json")
	fileCTExpr, _         = regexp.Compile("multipart/form-data|image|octet-stream")
	defaultBusinessCode   = "-1"
	version               = "0.0.8"
)

// Use set gin metrics middleware
func Use(r gin.IRoutes) {
	m := GetMonitor()
	m.initGinMetrics()
	m.setupServer()
	r.Use(m.monitorInterceptor)
}

// initGinMetrics used to init default metrics
func (m *Monitor) initGinMetrics() {
	// api耗时指标
	_ = monitor.AddMetric(&Metric{
		Type:        Histogram,
		Name:        metricRequestDuration,
		Description: "the time server took to handle the request.",
		Labels:      []string{"uri", "method", "httpcode", "code"},
		Buckets:     m.reqDuration,
	})
	// 慢请求指标
	_ = monitor.AddMetric(&Metric{
		Type:        Counter,
		Name:        metricSlowRequest,
		Description: fmt.Sprintf("the server handled slow requests counter, t=%d.", m.slowTime),
		Labels:      []string{"uri", "method", "httpcode", "code"},
	})

	_ = monitor.AddMetric(&Metric{
		Type:        Counter,
		Name:        metricLongRequest,
		Description: "the server handled long requests counter, like websocket、fileupload",
		Labels:      []string{"uri", "method", "httpcode", "code"},
	})

	_ = monitor.AddMetric(&Metric{
		Type:        Gauge,
		Name:        metricSDKVersion,
		Description: "current used monitor pkg version",
		Labels:      []string{"version"},
	})
	// 上报当前sdk版本
	_ = m.GetMetric(metricSDKVersion).SetGaugeValue([]string{version}, 1)
}

// monitorInterceptor as gin monitor middleware.
func (m *Monitor) monitorInterceptor(ctx *gin.Context) {
	startTime := time.Now()
	isSimpleRequest := true
	noFile := true
	writer := responseWriterPool.Get().(*responseWriter)

	reqHeaders := ctx.Request.Header
	// websocket请求
	if reqHeaders.Get("upgrade") == "websocket" {
		isSimpleRequest = false
	}
	// 文件上传请求
	if fileCTExpr.MatchString(reqHeaders.Get("content-type")) {
		noFile = false
		form, err := ctx.MultipartForm()
		// 如果上传文件数为0，当作简单请求处理
		if err == nil && len(form.File) == 0 {
			noFile = true
		}
	}
	writer.ResponseWriter = ctx.Writer
	ctx.Writer = writer

	// 执行请求其他流程
	ctx.Next()

	httpcode := ctx.Writer.Status()
	path := ctx.FullPath()
	method := ctx.Request.Method
	if path == "" {
		path = "unknow"
	}
	go func() {
		m.ginMetricHandle(path, method, isSimpleRequest, noFile, startTime, strconv.Itoa(httpcode), writer.code)
		writer.code = defaultBusinessCode
		responseWriterPool.Put(writer)
	}()
}

func (m *Monitor) ginMetricHandle(path string, method string, simpleRequest bool, noFile bool, start time.Time, httpcode string, code string) {
	// 共同的label
	labels := []string{path, method, httpcode, code}

	if simpleRequest && noFile {
		latency := time.Since(start).Seconds()
		if int32(latency) > m.slowTime {
			_ = m.GetMetric(metricSlowRequest).Inc(labels)
		}
		// set request duration
		_ = m.GetMetric(metricRequestDuration).Observe(labels, latency)
	} else {
		_ = m.GetMetric(metricLongRequest).Inc(labels)
	}
}
