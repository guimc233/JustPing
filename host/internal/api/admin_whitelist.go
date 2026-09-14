package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guimc233/JustPing/host/internal/auth"
	"github.com/guimc233/JustPing/host/internal/db"
	"github.com/guimc233/JustPing/host/internal/model"
)

type AddWhitelistReq struct {
	Email  string `json:"email" binding:"required"`
	Remark string `json:"remark"`
}

func adminListWhitelist(c *gin.Context) {
	var list []model.EmailWhitelist
	db.DB.Order("created_at desc").Find(&list)
	c.JSON(http.StatusOK, list)
}

func adminCreateWhitelist(c *gin.Context) {
	var req AddWhitelistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !strings.Contains(email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	val, _ := c.Get(auth.ContextUserKey)
	adminUser := val.(*model.User)

	item := model.EmailWhitelist{
		Email:     email,
		Remark:    strings.TrimSpace(req.Remark),
		CreatedBy: adminUser.Username,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already whitelisted"})
		return
	}

	c.JSON(http.StatusCreated, item)
}

func adminDeleteWhitelist(c *gin.Context) {
	id := c.Param("id")
	var item model.EmailWhitelist
	if err := db.DB.First(&item, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}

	var count int64
	db.DB.Model(&model.EmailWhitelist{}).Count(&count)
	if count <= 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete the last whitelisted email"})
		return
	}

	db.DB.Delete(&item)
	c.JSON(http.StatusOK, gin.H{"message": "Email removed from whitelist"})
}
