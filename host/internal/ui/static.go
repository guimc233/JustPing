package ui

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var embeddedDist embed.FS

// SetupStaticServ serves UI static assets securely, trying disk first, then embedded dist
func SetupStaticServ(r *gin.Engine) {
	diskDist := findDiskDist()
	var subFS fs.FS
	if diskDist != "" {
		log.Printf("[Web] Serving UI from local disk: %s\n", diskDist)
	} else {
		var err error
		subFS, err = fs.Sub(embeddedDist, "dist")
		if err == nil {
			if _, statErr := fs.Stat(subFS, "index.html"); statErr == nil {
				log.Printf("[Web] Serving UI from embedded binary assets\n")
			} else {
				subFS = nil
			}
		}
	}

	if diskDist == "" && subFS == nil {
		r.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"service": "JustPing Host API",
				"version": "1.0.0",
				"status":  "running",
				"hint":    "Frontend assets not found.",
			})
		})
		return
	}

	r.Use(func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		if strings.HasPrefix(reqPath, "/api") || reqPath == "/install.sh" {
			c.Next()
			return
		}

		cleanPath := path.Clean("/" + reqPath)
		trimmed := strings.TrimPrefix(cleanPath, "/")

		if diskDist != "" {
			serveFromDisk(c, diskDist, trimmed)
		} else {
			serveFromFS(c, subFS, trimmed)
		}
	})
}

func serveFromDisk(c *gin.Context, diskDist, relPath string) {
	absDist, err := filepath.Abs(diskDist)
	if err != nil {
		c.Next()
		return
	}

	target := filepath.Join(absDist, filepath.FromSlash(relPath))
	targetClean := filepath.Clean(target)
	if !strings.HasPrefix(targetClean, absDist+string(filepath.Separator)) && targetClean != absDist {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	if info, err := os.Stat(targetClean); err == nil && !info.IsDir() {
		c.File(targetClean)
		c.Abort()
		return
	}

	indexPath := filepath.Join(absDist, "index.html")
	if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
		c.File(indexPath)
		c.Abort()
		return
	}
	c.Next()
}

func serveFromFS(c *gin.Context, subFS fs.FS, relPath string) {
	if relPath != "" {
		if f, err := subFS.Open(relPath); err == nil {
			defer f.Close()
			if stat, err := f.Stat(); err == nil && !stat.IsDir() {
				http.FileServer(http.FS(subFS)).ServeHTTP(c.Writer, c.Request)
				c.Abort()
				return
			}
		}
	}

	if f, err := subFS.Open("index.html"); err == nil {
		defer f.Close()
		stat, _ := f.Stat()
		data, err := io.ReadAll(f)
		if err == nil {
			c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), bytes.NewReader(data))
			c.Abort()
			return
		}
	}
	c.Next()
}

func findDiskDist() string {
	candidates := []string{"web/dist", "../web/dist", "/app/web/dist", "dist"}
	for _, dir := range candidates {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
				return dir
			}
		}
	}
	return ""
}
