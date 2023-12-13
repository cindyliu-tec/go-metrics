package go_metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"git.makeblock.com/makeblock-go/utils/v2/response"
	"github.com/gin-gonic/gin"
)

var (
	metricRequestDuration = "gin_uri_request_duration"
	metricSlowRequest     = "gin_slow_request_total"
	metricLongRequest     = "gin_long_request_total"
	metricSDKVersion      = "monitor_sdk_version"
	jsonCTExpr, _         = regexp.Compile("application/json")
	fileCTExpr, _         = regexp.Compile("multipart/form-data｜image｜octet-stream")
	version               = "0.0.1"
)

// Use set gin metrics middleware
func Use(r gin.IRoutes) {
	m := GetMonitor()
	m.initGinMetrics()
	err := m.setupServer()
	if err == nil {
		r.Use(m.monitorInterceptor)
	}
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

type responseWriter struct {
	gin.ResponseWriter
	b *bytes.Buffer
}

func (w responseWriter) Write(b []byte) (int, error) {
	w.b.Write(b)
	return w.ResponseWriter.Write(b)
}

// monitorInterceptor as gin monitor middleware.
func (m *Monitor) monitorInterceptor(ctx *gin.Context) {
	startTime := time.Now()
	isSimpleRequest := true
	var writer *responseWriter
	var code = "-1"
	// websocket请求和文件上传请求不处理
	if fileCTExpr.MatchString(ctx.Writer.Header().Get("content-type")) || ctx.Request.Header.Get("upgrade") == "websocket" {
		isSimpleRequest = false
		form, err := ctx.MultipartForm()
		// 如果上传文件数为0，当作简单请求处理
		if err == nil && len(form.File) == 0 {
			isSimpleRequest = true
		}
	}
	if isSimpleRequest {
		writer = &responseWriter{
			ResponseWriter: ctx.Writer,
			b:              bytes.NewBuffer([]byte{}),
		}
		ctx.Writer = writer
	}

	// 执行请求其他流程
	ctx.Next()

	// 简单请求如果返回json，读取业务code
	if isSimpleRequest && jsonCTExpr.MatchString(ctx.Writer.Header().Get("content-type")) {
		var data = response.APIModel{
			Code: -1,
		}
		if err := json.Unmarshal(writer.b.Bytes(), &data); err == nil {
			code = strconv.Itoa(int(data.Code))
		}
		writer.b = nil
	}
	httpcode := ctx.Writer.Status()
	m.ginMetricHandle(ctx, isSimpleRequest, startTime, strconv.Itoa(httpcode), code)
}

func (m *Monitor) ginMetricHandle(ctx *gin.Context, simpleRequest bool, start time.Time, httpcode string, code string) {
	// 共同的label
	labels := []string{ctx.FullPath(), ctx.Request.Method, httpcode, code}

	if simpleRequest {
		latency := time.Since(start)
		if int32(latency.Seconds()) > m.slowTime {
			_ = m.GetMetric(metricSlowRequest).Inc(labels)
		}

		// set request duration
		_ = m.GetMetric(metricRequestDuration).Observe(labels, latency.Seconds())
	} else {
		_ = m.GetMetric(metricLongRequest).Inc(labels)
	}
}
