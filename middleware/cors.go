package middleware

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowOriginFunc = func(origin string) bool {
		return true
	}
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	// Older Safari/WebKit builds can be picky about wildcard request headers in CORS preflight.
	// Keep the common API/auth headers explicit so iOS browsers do not reject the request at the gate.
	config.AllowHeaders = []string{
		"Origin",
		"Content-Length",
		"Content-Type",
		"Accept",
		"Authorization",
		"X-Requested-With",
		"X-API-Key",
		"X-Api-Key",
		"x-api-key",
		"x-goog-api-key",
		"Anthropic-Version",
		"Anthropic-Beta",
		"OpenAI-Beta",
		"OpenAI-Organization",
		"OpenAI-Project",
	}
	return cors.New(config)
}

func PoweredBy() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
