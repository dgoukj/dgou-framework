package monitor

import (
	"expvar"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	ServiceName     string
	ServiceVersion  string
	Environment     string
	EnableMetrics   bool
	MetricsPath     string
	EnableHealth    bool
	HealthPath      string
	EnableProfiling bool
	ProfilePath     string
}

type Monitor struct {
	config       Config
	registry     *prometheus.Registry
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	startTime    time.Time
	mu           sync.RWMutex
}

func New(cfg Config) *Monitor {
	m := &Monitor{
		config:    cfg,
		registry:  prometheus.NewRegistry(),
		startTime: time.Now(),
	}
	if cfg.EnableMetrics {
		m.initMetrics()
	}
	return m
}

func (m *Monitor) initMetrics() {
	m.httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
			ConstLabels: prometheus.Labels{
				"service": m.config.ServiceName,
			},
		},
		[]string{"method", "path", "status"},
	)
	m.httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	m.registry.MustRegister(m.httpRequests, m.httpDuration)
	m.registry.MustRegister(prometheus.NewGoCollector())
	m.registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
}

func (m *Monitor) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Monitor) MetricsPath() string {
	return m.config.MetricsPath
}

func (m *Monitor) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"healthy","service":"%s","timestamp":%d}`, m.config.ServiceName, time.Now().Unix())
	}
}

func (m *Monitor) RegisterPprof(mux *http.ServeMux) {
	mux.HandleFunc(m.config.ProfilePath+"/", pprof.Index)
	mux.HandleFunc(m.config.ProfilePath+"/cmdline", pprof.Cmdline)
	mux.HandleFunc(m.config.ProfilePath+"/profile", pprof.Profile)
	mux.HandleFunc(m.config.ProfilePath+"/symbol", pprof.Symbol)
	mux.HandleFunc(m.config.ProfilePath+"/trace", pprof.Trace)
	mux.Handle(m.config.ProfilePath+"/allocs", pprof.Handler("allocs"))
	mux.Handle(m.config.ProfilePath+"/block", pprof.Handler("block"))
	mux.Handle(m.config.ProfilePath+"/goroutine", pprof.Handler("goroutine"))
	mux.Handle(m.config.ProfilePath+"/heap", pprof.Handler("heap"))
	mux.Handle(m.config.ProfilePath+"/mutex", pprof.Handler("mutex"))
	mux.Handle(m.config.ProfilePath+"/threadcreate", pprof.Handler("threadcreate"))
	mux.HandleFunc(m.config.ProfilePath+"/vars", expvar.Handler().ServeHTTP)
}

// 中间件适配器：返回Gin HandlerFunc
func (m *Monitor) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if m.config.EnableMetrics && m.httpRequests != nil {
			path := c.FullPath()
			if path == "" {
				path = "unknown"
			}
			status := fmt.Sprintf("%d", c.Writer.Status())
			m.httpRequests.WithLabelValues(c.Request.Method, path, status).Inc()
			m.httpDuration.WithLabelValues(c.Request.Method, path, status).Observe(time.Since(start).Seconds())
		}
	}
}
