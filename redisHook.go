package go_metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type MonitorRedisHook struct{}

func (h *MonitorRedisHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, "start_time", time.Now()), nil
}

func (h *MonitorRedisHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	start := ctx.Value("start_time").(time.Time)
	duration := time.Since(start).Microseconds()
	fmt.Printf("redis command [%s] took [%dμs] to process\n", cmd.String(), duration)
	_ = monitor.GetMetric(metricRedisDuration).Observe([]string{cmd.Name()}, float64(duration))
	return nil
}

func (h *MonitorRedisHook) BeforeProcessPipeline(ctx context.Context, cmds []redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, "start_time", time.Now()), nil
}

func (h *MonitorRedisHook) AfterProcessPipeline(ctx context.Context, cmds []redis.Cmder) error {
	start := ctx.Value("start_time").(time.Time)
	duration := time.Since(start).Microseconds()
	fmt.Printf("redis pipeline [%v] took [%dμs] to process\n", cmds, duration)
	_ = monitor.GetMetric(metricRedisDuration).Observe([]string{"pipeline"}, float64(duration))
	return nil
}
