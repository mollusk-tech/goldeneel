package routes

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"messagingapi/internal/config"
)

func RegisterMediaRoutes(r *gin.RouterGroup, db *sql.DB, cfg config.Config) {
	r.GET("/avatar", func(c *gin.Context) {
		uid := c.GetString("userID")
		var p string
		if err := db.QueryRow(`SELECT avatar_path FROM users WHERE id=$1`, uid).Scan(&p); err != nil || p == "" {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.File(p)
	})

	r.GET("/attachments/:id", func(c *gin.Context) {
		uid := c.GetString("userID")
		attID := c.Param("id")
		var filePath, chatID string
		err := db.QueryRow(`SELECT a.file_path, m.chat_id FROM attachments a JOIN messages m ON a.message_id=m.id WHERE a.id=$1`, attID).Scan(&filePath, &chatID)
		if err != nil { c.AbortWithStatus(http.StatusNotFound); return }
		var member int
		if err := db.QueryRow(`SELECT COUNT(*) FROM chat_members WHERE chat_id=$1 AND user_id=$2`, chatID, uid).Scan(&member); err != nil || member == 0 {
			c.AbortWithStatus(http.StatusForbidden); return
		}
		// set content-type by extension best-effort
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".jpg", ".jpeg": c.Header("Content-Type", "image/jpeg")
		case ".png": c.Header("Content-Type", "image/png")
		case ".gif": c.Header("Content-Type", "image/gif")
		case ".mp4": c.Header("Content-Type", "video/mp4")
		}
		c.File(filePath)
	})
}