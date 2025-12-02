package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(cors.Default())
	r.Use(cors.New(cors.Config{
		AllowOrigins:  []string{"http://localhost:5173", "http://localhost:5174"},
		AllowMethods:  []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Authorization", "trace_id"},
		ExposeHeaders: []string{"trace_id"},
		MaxAge:        12 * time.Hour,
	}))

	services := map[string]string{
		"user": "http://localhost:8081/user",
		"sprint": "http://localhost:8083/sprint",
		"task" : "http://localhost:8082/task",
	}

	// STEP 1: Register ALL specific routes FIRST
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":   "gateway is healthy",
			"time":     c.GetTime("time"),
			"services": keys(services),
		})
	})

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message":  "Welcome to API Gateway",
			"usage":    "POST/GET /<service-name>/path",
			"example":  "/user-service/profile",
			"health":   "/health",
			"services": keys(services),
		})
	})

	// STEP 2: Register the catch-all proxy LAST
	r.NoRoute(func(ctx *gin.Context) { // Use NoRoute instead of Any("/*path")
		path := strings.Trim(ctx.Request.URL.Path, "/")
		parts := strings.Split(path, "/")
		fmt.Printf("[Gateway] %s %s → %v\n", ctx.Request.Method, ctx.Request.URL.Path, parts)

		if len(parts) == 0 || parts[0] == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "service name required"})
			return
		}

		serviceName := parts[0]
		targetURL, exists := services[serviceName]
		if !exists {
			ctx.JSON(http.StatusNotFound, gin.H{
				"error":     "service not found",
				"requested": serviceName,
				"available": keys(services),
			})
			return
		}

		target, err := url.Parse(targetURL)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "invalid backend URL"})
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		proxy.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Del("Access-Control-Allow-Origin")
			resp.Header.Del("Access-Control-Allow-Credentials")
			resp.Header.Del("Access-Control-Allow-Headers")
			resp.Header.Del("Access-Control-Allow-Methods")
			resp.Header.Del("Access-Control-Expose-Headers")
			return nil
		}

		// Strip the service prefix
		if len(parts) > 1 {
			ctx.Request.URL.Path = "/" + strings.Join(parts[1:], "/")
		} else {
			ctx.Request.URL.Path = "/"
		}

		// Fix host and forwarded headers
		ctx.Request.Host = target.Host
		ctx.Request.Header.Set("X-Forwarded-Host", ctx.Request.Host)
		ctx.Request.Header.Set("X-Forwarded-Proto", "http")
		ctx.Request.Header.Set("X-Forwarded-For", ctx.ClientIP())

		proxy.ServeHTTP(ctx.Writer, ctx.Request)
	})

	fmt.Println("API Gateway running on http://localhost:8080")
	r.Run(":8080")
}

func keys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
