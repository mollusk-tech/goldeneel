package routes

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"messagingapi/internal/auth"
	"messagingapi/internal/config"
)

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	OldPIN      string `json:"oldPin" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
	NewPIN      string `json:"newPin" binding:"required,min=4,max=10"`
}

func RegisterUserRoutes(r *gin.RouterGroup, db *sql.DB, cfg config.Config) {
	r.GET("/me", func(c *gin.Context) {
		uid := c.GetString("userID")
		var u struct{
			ID string `json:"id"`
			Username string `json:"username"`
			DisplayName string `json:"displayName"`
			AvatarPath sql.NullString `json:"-"`
			PublicKey string `json:"publicKey"`
			LastActiveAt sql.NullTime `json:"-"`
		}
		err := db.QueryRow(`SELECT id, username, display_name, avatar_path, public_key, last_active_at FROM users WHERE id=$1`, uid).Scan(&u.ID,&u.Username,&u.DisplayName,&u.AvatarPath,&u.PublicKey,&u.LastActiveAt)
		if err != nil { c.JSON(http.StatusNotFound, gin.H{"error":"not found"}); return }
		resp := gin.H{
			"id": u.ID,
			"username": u.Username,
			"displayName": u.DisplayName,
			"publicKey": u.PublicKey,
		}
		if u.AvatarPath.Valid { resp["avatarUrl"] = "/api/v1/media/avatar" }
		if u.LastActiveAt.Valid { resp["lastActiveAt"] = u.LastActiveAt.Time }
		c.JSON(http.StatusOK, resp)
	})

	r.PUT("/me/password", func(c *gin.Context) {
		uid := c.GetString("userID")
		var req changePasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
		var oldPwdHash, oldPinHash string
		if err := db.QueryRow(`SELECT password_hash, pin_hash FROM users WHERE id=$1`, uid).Scan(&oldPwdHash, &oldPinHash); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error":"user"}); return }
		ok,_ := auth.VerifyPassword(oldPwdHash, req.OldPassword); if !ok { c.JSON(http.StatusUnauthorized, gin.H{"error":"invalid"}); return }
		ok,_ = auth.VerifyPassword(oldPinHash, req.OldPIN); if !ok { c.JSON(http.StatusUnauthorized, gin.H{"error":"invalid"}); return }
		newPwd, _ := auth.HashPassword(req.NewPassword)
		newPin, _ := auth.HashPassword(req.NewPIN)
		_, err := db.Exec(`UPDATE users SET password_hash=$1, pin_hash=$2, updated_at=now() WHERE id=$3`, newPwd, newPin, uid)
		if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error":"update"}); return }
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	r.POST("/me/avatar", func(c *gin.Context) {
		uid := c.GetString("userID")
		file, err := c.FormFile("avatar")
		if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error":"avatar required"}); return }
		dir := filepath.Join(cfg.DataDir, "avatars", uid)
		if err := os.MkdirAll(dir, 0755); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error":"mkdir"}); return }
		ext := filepath.Ext(file.Filename)
		name := fmt.Sprintf("avatar%s", ext)
		path := filepath.Join(dir, name)
		if err := c.SaveUploadedFile(file, path); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error":"save"}); return }
		_, _ = db.Exec(`UPDATE users SET avatar_path=$1, updated_at=now() WHERE id=$2`, path, uid)
		c.JSON(http.StatusOK, gin.H{"avatarUrl": "/api/v1/media/avatar"})
	})
}