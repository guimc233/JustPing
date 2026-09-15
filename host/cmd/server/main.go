package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/api"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/ui"
)

func main() {
	log.Println("Starting JustPing Host...")

	if _, err := db.InitDB(); err != nil {
		log.Fatalf("Fatal: Database initialization failed: %v", err)
	}

	db.StartRetentionCleaner(12 * time.Hour)

	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(ginMode)
	}

	r := gin.Default()
	_ = r.SetTrustedProxies(nil) // Restrict default reverse proxy trusts

	// Explicit secure CORS
	appURL := db.GetSetting("app_url")
	if appURL == "" {
		appURL = os.Getenv("APP_URL")
	}
	allowedOrigins := []string{
		"http://localhost:5173",
		"http://localhost:8080",
		"http://127.0.0.1:5173",
		"http://127.0.0.1:8080",
	}
	if appURL != "" {
		allowedOrigins = append(allowedOrigins, strings.TrimRight(appURL, "/"))
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Register API endpoints
	api.RegisterRoutes(r)

	// Serve install.sh dynamically
	r.GET("/install.sh", serveInstallScript)

	// Serve Web static files with SPA fallback (embedded + disk check)
	ui.SetupStaticServ(r)

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

	c.Header("Content-Type", "text/x-shellscript; charset=utf-8")
	c.String(http.StatusOK, "#!/bin/sh\necho 'JustPing agent installer'\n")
}
