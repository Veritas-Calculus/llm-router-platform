package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
}

func TestVersionEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/version", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version":    Version,
			"git_commit": GitCommit,
			"build_time": BuildTime,
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/version", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["version"] != "dev" {
		t.Errorf("expected version 'dev', got %q", body["version"])
	}
	if body["git_commit"] != "unknown" {
		t.Errorf("expected git_commit 'unknown', got %q", body["git_commit"])
	}
}

func TestServicesStruct(t *testing.T) {
	// Ensure Services struct can be instantiated with zero values
	s := &Services{}
	if s.User != nil || s.Router != nil {
		t.Error("zero-value Services should have nil fields")
	}
}

func TestBuildVars(t *testing.T) {
	// Default build vars should have expected values
	if Version != "dev" {
		t.Errorf("expected Version 'dev', got %q", Version)
	}
	if GitCommit != "unknown" {
		t.Errorf("expected GitCommit 'unknown', got %q", GitCommit)
	}
	if BuildTime != "unknown" {
		t.Errorf("expected BuildTime 'unknown', got %q", BuildTime)
	}
}
