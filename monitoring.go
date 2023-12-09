package go_metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"git.makeblock.com/makeblock-go/utils/v2/response"
	"github.com/gin-gonic/gin"
)

var (
	metricRequestDuration = "gin_uri_request_duration"
	metricSlowRequest     = "gin_slow_request_total"
	env                   = os.Getenv("PROJECT_ENV")
	pod                   = os.Getenv("HOSTNAME")
	app                   = os.Getenv("APP_NAME")
	ctExpr, _             = regexp.Compile("application/json")
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
	_ = monitor.AddMetric(&Metric{
		Type:        Histogram,
		Name:        metricRequestDuration,
		Description: "the time server took to handle the request.",
		Labels:      []string{"env", "app", "pod", "uri", "method", "code"},
		Buckets:     m.reqDuration,
	})
	_ = monitor.AddMetric(&Metric{
		Type:        Counter,
		Name:        metricSlowRequest,
		Description: fmt.Sprintf("the server handled slow requests counter, t=%d.", m.slowTime),
		Labels:      []string{"env", "app", "pod", "uri", "method", "code"},
	})
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
	writer := responseWriter{
		ResponseWriter: ctx.Writer,
		b:              bytes.NewBuffer([]byte{}),
	}
	ctx.Writer = writer
	// 执行请求其他流程
	ctx.Next()

	// 读取业务code
	var code = "-1"
	if ctExpr.MatchString(ctx.Request.Response.Header.Get("content-type")) {
		data := response.APIModel{}
		if err := json.Unmarshal(writer.b.Bytes(), &data); err == nil {
			code = strconv.Itoa(int(data.Code))
		}
	}
	writer.b = nil
	m.ginMetricHandle(ctx, startTime, code)
}

func (m *Monitor) ginMetricHandle(ctx *gin.Context, start time.Time, code string) {
	r := ctx.Request
	// set slow request
	latency := time.Since(start)
	if int32(latency.Seconds()) > m.slowTime {
		_ = m.GetMetric(metricSlowRequest).Inc([]string{env, app, pod, ctx.FullPath(), r.Method, code})
	}

	// set request duration
	_ = m.GetMetric(metricRequestDuration).Observe([]string{env, app, pod, ctx.FullPath(), r.Method, code}, latency.Seconds())
}
