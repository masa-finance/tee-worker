package api_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/api"
	"github.com/masa-finance/tee-worker/internal/jobserver"
)

var _ = Describe("Health Checks", func() {
	Describe("HealthMetrics", func() {
		var hm *HealthMetrics

		BeforeEach(func() {
			hm = NewHealthMetrics()
		})

		It("should initialize with correct defaults", func() {
			Expect(hm).NotTo(BeNil())
			stats := hm.GetStats()
			Expect(stats["error_count"]).To(Equal(0))
			Expect(stats["success_count"]).To(Equal(0))
			Expect(stats["window_duration"]).To(Equal("10m0s"))
		})

		It("should record successes", func() {
			hm.RecordSuccess()
			hm.RecordSuccess()

			stats := hm.GetStats()
			Expect(stats["success_count"]).To(Equal(2))
			Expect(stats["error_count"]).To(Equal(0))
		})

		It("should record errors", func() {
			hm.RecordError()
			hm.RecordError()
			hm.RecordError()

			stats := hm.GetStats()
			Expect(stats["success_count"]).To(Equal(0))
			Expect(stats["error_count"]).To(Equal(3))
		})

		Context("IsHealthy", func() {
			It("should be healthy with no requests", func() {
				Expect(hm.IsHealthy()).To(BeTrue())
			})

			It("should be healthy with low error rate", func() {
				// 5% error rate (healthy)
				for i := 0; i < 95; i++ {
					hm.RecordSuccess()
				}
				for i := 0; i < 5; i++ {
					hm.RecordError()
				}

				Expect(hm.IsHealthy()).To(BeTrue())
			})

			It("should be unhealthy with high error rate", func() {
				// 96% error rate (unhealthy)
				for i := 0; i < 4; i++ {
					hm.RecordSuccess()
				}
				for i := 0; i < 96; i++ {
					hm.RecordError()
				}

				Expect(hm.IsHealthy()).To(BeFalse())
			})
		})

		It("should calculate stats correctly", func() {
			hm.RecordSuccess()
			hm.RecordSuccess()
			hm.RecordError()

			stats := hm.GetStats()
			Expect(stats["error_count"]).To(Equal(1))
			Expect(stats["success_count"]).To(Equal(2))
			Expect(stats["total_count"]).To(Equal(3))
			Expect(stats["error_rate"]).To(BeNumerically("~", 0.333, 0.01))
		})
	})

	Describe("Healthz Endpoint", func() {
		It("should return 200 OK", func() {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := Healthz()
			err := handler(c)

			Expect(err).To(BeNil())
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(ContainSubstring(`"status":"ok"`))
			Expect(rec.Body.String()).To(ContainSubstring(`"service":"tee-worker"`))
		})
	})

	Describe("Readyz Endpoint", func() {
		var (
			e         *echo.Echo
			jobServer *jobserver.JobServer
			hm        *HealthMetrics
		)

		BeforeEach(func() {
			e = echo.New()
			hm = NewHealthMetrics()
		})

		Context("when all checks pass", func() {
			It("should return 200 OK", func() {
				jobServer = jobserver.NewJobServer(10, types.JobConfiguration{})

				// Record mostly successes
				for i := 0; i < 95; i++ {
					hm.RecordSuccess()
				}
				for i := 0; i < 5; i++ {
					hm.RecordError()
				}

				req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)

				handler := Readyz(jobServer, hm)
				err := handler(c)

				Expect(err).To(BeNil())
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring(`"ready":true`))
				Expect(rec.Body.String()).To(ContainSubstring(`"job_server":"ok"`))
				Expect(rec.Body.String()).To(ContainSubstring(`"error_rate":"healthy"`))
			})
		})

		Context("when job server is nil", func() {
			It("should return 503 Service Unavailable", func() {
				req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)

				handler := Readyz(nil, hm)
				err := handler(c)

				Expect(err).To(BeNil())
				Expect(rec.Code).To(Equal(http.StatusServiceUnavailable))
				Expect(rec.Body.String()).To(ContainSubstring(`"ready":false`))
				Expect(rec.Body.String()).To(ContainSubstring(`"job_server":"not initialized"`))
			})
		})

		Context("when error rate is high", func() {
			It("should return 503 Service Unavailable", func() {
				jobServer = jobserver.NewJobServer(10, types.JobConfiguration{})

				// Record mostly errors
				for i := 0; i < 4; i++ {
					hm.RecordSuccess()
				}
				for i := 0; i < 96; i++ {
					hm.RecordError()
				}

				req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)

				handler := Readyz(jobServer, hm)
				err := handler(c)

				Expect(err).To(BeNil())
				Expect(rec.Code).To(Equal(http.StatusServiceUnavailable))
				Expect(rec.Body.String()).To(ContainSubstring(`"ready":false`))
				Expect(rec.Body.String()).To(ContainSubstring(`"error_rate":"unhealthy"`))
				Expect(rec.Body.String()).To(ContainSubstring(`"error_count":96`))
			})
		})
	})

	Describe("HealthMetricsMiddleware", func() {
		var (
			e  *echo.Echo
			hm *HealthMetrics
		)

		BeforeEach(func() {
			e = echo.New()
			hm = NewHealthMetrics()
			e.Use(HealthMetricsMiddleware(hm))
		})

		It("should skip health endpoints", func() {
			e.GET("/healthz", func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			stats := hm.GetStats()
			Expect(stats["success_count"]).To(Equal(0))
			Expect(stats["error_count"]).To(Equal(0))
		})

		It("should record success for 2xx response", func() {
			e.POST("/job/add", func(c echo.Context) error {
				return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			req := httptest.NewRequest(http.MethodPost, "/job/add", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			stats := hm.GetStats()
			Expect(stats["success_count"]).To(Equal(1))
			Expect(stats["error_count"]).To(Equal(0))
		})

		It("should record error for 5xx response", func() {
			e.POST("/job/add", func(c echo.Context) error {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "server error"})
			})

			req := httptest.NewRequest(http.MethodPost, "/job/add", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			stats := hm.GetStats()
			Expect(stats["success_count"]).To(Equal(0))
			Expect(stats["error_count"]).To(Equal(1))
		})

		It("should skip 4xx client errors", func() {
			e.POST("/job/add", func(c echo.Context) error {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
			})

			req := httptest.NewRequest(http.MethodPost, "/job/add", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			stats := hm.GetStats()
			Expect(stats["success_count"]).To(Equal(0))
			Expect(stats["error_count"]).To(Equal(0))
		})

		It("should skip non-API endpoints", func() {
			e.GET("/static/file.css", func(c echo.Context) error {
				return c.String(http.StatusOK, "css content")
			})

			req := httptest.NewRequest(http.MethodGet, "/static/file.css", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			stats := hm.GetStats()
			Expect(stats["success_count"]).To(Equal(0))
			Expect(stats["error_count"]).To(Equal(0))
		})
	})
})
