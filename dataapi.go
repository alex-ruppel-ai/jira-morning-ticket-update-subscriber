package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

var iamTokenSource oauth2.TokenSource

func dataAPIURL() string {
	base := os.Getenv("URL_BASE")
	var url string
	if base == "" {
		url = "http://localhost:8080"
	} else {
		url = "https://dataapi." + base
	}
	zap.L().Debug("data API URL resolved",
		zap.String("url_base", base),
		zap.String("data_api_url", url),
	)
	return url
}

func initIDTokenSource() {
	audience := dataAPIURL()
	zap.L().Info("initializing ID token source", zap.String("audience", audience))
	ts, err := idtoken.NewTokenSource(context.Background(), audience)
	if err != nil {
		zap.L().Warn("could not create ID token source — IAM auth disabled",
			zap.String("audience", audience),
			zap.Error(err),
		)
		return
	}
	iamTokenSource = ts
	zap.L().Info("ID token source initialized", zap.String("audience", audience))
}

func forwardToDataAPI(c *gin.Context, method, path string, body io.Reader) {
	targetURL := dataAPIURL() + path
	token := c.GetHeader("X-Request-Token")

	zap.L().Info("forwarding request to data API",
		zap.String("method", method),
		zap.String("path", path),
		zap.String("target_url", targetURL),
	)

	req, err := http.NewRequestWithContext(c.Request.Context(), method, targetURL, body)
	if err != nil {
		zap.L().Error("failed to build data API request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
		return
	}
	req.Header.Set("X-Request-Token", token)
	if ct := c.Request.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if iamTokenSource != nil {
		idToken, err := iamTokenSource.Token()
		if err != nil {
			zap.L().Error("failed to mint IAM token", zap.Error(err))
		} else {
			zap.L().Info("setting IAM token", zap.String("token", idToken.AccessToken))
			req.Header.Set("Authorization", "Bearer "+idToken.AccessToken)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if isLocalDev() {
			zap.L().Warn("data API unavailable in local development — integrations require deployment",
				zap.String("hint", "deploy with: apps-platform app deploy"),
			)
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":             "Integrations are not available in local development.",
				"local_development": true,
			})
		} else {
			zap.L().Error("data API unreachable", zap.Error(err))
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("data api unreachable: %v", err)})
		}
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		zap.L().Error("data API returned error",
			zap.Int("status", resp.StatusCode),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("response", string(respBody)),
		)
	}
	c.Status(resp.StatusCode)
	for k, vs := range resp.Header {
		for _, v := range vs {
			c.Header(k, v)
		}
	}
	c.Writer.Write(respBody)
}

func isLocalDev() bool {
	return os.Getenv("URL_BASE") == ""
}

func registerDataAPIRoutes(api *gin.RouterGroup) {
	api.POST("/connect/:integration", func(c *gin.Context) {
		integration := c.Param("integration")
		forwardToDataAPI(c, "POST", "/api/data/oauth/start?integration="+integration, nil)
	})
	api.GET("/connections", func(c *gin.Context) {
		forwardToDataAPI(c, "GET", "/api/data/connections", nil)
	})
	api.Any("/integration/*path", func(c *gin.Context) {
		path := "/api/data" + c.Param("path")
		if q := c.Request.URL.RawQuery; q != "" {
			path += "?" + q
		}
		forwardToDataAPI(c, c.Request.Method, path, c.Request.Body)
	})
}
