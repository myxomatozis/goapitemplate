package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"goapitemplate/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		logrus.WithFields(logrus.Fields{
			"status_code":  param.StatusCode,
			"latency":      param.Latency,
			"client_ip":    param.ClientIP,
			"method":       param.Method,
			"path":         param.Path,
			"user_agent":   param.Request.UserAgent(),
			"error":        param.ErrorMessage,
			"body_size":    param.BodySize,
		}).Info("HTTP Request")
		return ""
	})
}

func Recovery() gin.HandlerFunc {
	return gin.RecoveryWithWriter(gin.DefaultErrorWriter, func(c *gin.Context, recovered interface{}) {
		logrus.WithField("panic", recovered).Error("Panic recovered")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
	})
}

func CORS(corsConfig config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Check if origin is allowed
		if len(corsConfig.AllowedOrigins) > 0 && !contains(corsConfig.AllowedOrigins, "*") {
			if !contains(corsConfig.AllowedOrigins, origin) {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		// Set CORS headers
		if contains(corsConfig.AllowedOrigins, "*") {
			c.Header("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		if corsConfig.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if len(corsConfig.AllowedHeaders) > 0 {
			c.Header("Access-Control-Allow-Headers", strings.Join(corsConfig.AllowedHeaders, ", "))
		}

		if len(corsConfig.AllowedMethods) > 0 {
			c.Header("Access-Control-Allow-Methods", strings.Join(corsConfig.AllowedMethods, ", "))
		}

		if corsConfig.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", corsConfig.MaxAge))
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func RateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	type client struct {
		requests int
		window   time.Time
	}

	clients := make(map[string]*client)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		if clientData, exists := clients[ip]; exists {
			if time.Since(clientData.window) > window {
				clientData.requests = 1
				clientData.window = time.Now()
			} else {
				clientData.requests++
				if clientData.requests > maxRequests {
					c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
						"error": "Rate limit exceeded",
					})
					return
				}
			}
		} else {
			clients[ip] = &client{
				requests: 1,
				window:   time.Now(),
			}
		}

		c.Next()
	}
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[i%len(charset)]
	}
	return string(result)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}