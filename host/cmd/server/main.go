package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/api"
	"github.com/guimc233/JustPing/host/internal/db"
)

func main() {
	log.Println("Starting JustPing Host...")

	// 1. Initialize Database
	if _, err := db.InitDB(); err != nil {
		log.Fatalf("Fatal: Database initialization failed: %v", err)
	}

	// 2. Start Background Retention Cleaner
	db.StartRetentionCleaner(12 * time.Hour)

	// 3. Initialize Gin Engine
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(ginMode)
	}

	r := gin.Default()

	// CORS config (allows Vite dev server in local testing)
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Register API endpoints
	api.RegisterRoutes(r)

	// Serve install.sh dynamically
	r.GET("/install.sh", serveInstallScript)

	// Serve Web static files with SPA fallback
	setupStaticFiles(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("JustPing Host is listening on :%s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}

func serveInstallScript(c *gin.Context) {
	// Look for scripts/install.sh
	possiblePaths := []string{
		"scripts/install.sh",
		"../scripts/install.sh",
		"/app/scripts/install.sh",
	}

	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			c.Header("Content-Type", "text/x-shellscript; charset=utf-8")
			c.File(p)
			return
		}
	}

	// Fallback dynamic script if file not on disk
	c.Header("Content-Type", "text/x-shellscript; charset=utf-8")
	c.String(http.StatusOK, "#!/bin/sh\necho 'JustPing agent installer'\n")
}

func setupStaticFiles(r *gin.Engine) {
	distDirs := []string{
		"web/dist",
		"../web/dist",
		"/app/web/dist",
		"dist",
	}

	var foundDist string
	for _, dir := range distDirs {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			foundDist = dir
			break
		}
	}

	if foundDist != "" {
		log.Printf("[Web] Serving static assets from %s\n", foundDist)
		r.Use(func(c *gin.Context) {
			path := c.Request.URL.Path
			// Skip API, WebSocket, and install.sh routes
			if strings.HasPrefix(path, "/api") || path == "/install.sh" {
				c.Next()
				return
			}

			filePath := filepath.Join(foundDist, filepath.Clean(path))
			if stat, err := os.Stat(filePath); err == nil && !stat.IsDir() {
				c.File(filePath)
				c.Abort()
				return
			}

			// SPA Fallback: serve index.html for non-asset routes
			indexFile := filepath.Join(foundDist, "index.html")
			if _, err := os.Stat(indexFile); err == nil {
				c.File(indexFile)
				c.Abort()
				return
			}
			c.Next()
		})
	} else {
		r.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"service": "JustPing Host API",
				"version": "1.0.0",
				"status":  "running",
				"hint":    "Frontend assets not found in dist. Run Vite frontend build.",
			})
		})
	}
}
