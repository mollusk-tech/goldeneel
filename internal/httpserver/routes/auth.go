package routes

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"messagingapi/internal/auth"
	"messagingapi/internal/config"
)

type registerRequest struct {
	InviteCode string `json:"inviteCode" binding:"required"`
	Username   string `json:"username" binding:"required,min=3"`
	DisplayName string `json:"displayName" binding:"required"`
	Password   string `json:"password" binding:"required,min=6"`
	PIN        string `json:"pin" binding:"required,min=4,max=10"`
	PublicKey  string `json:"publicKey" binding:"required"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	PIN      string `json:"pin" binding:"required"`
}

func RegisterAuthRoutes(r *gin.RouterGroup, db *sql.DB, cfg config.Config) {
	r.POST("/register", func(c *gin.Context) {
		var req registerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var inviteID string
		var maxUses, uses int
		var active bool
		var expiresAt sql.NullTime
		err := db.QueryRow(`SELECT id, max_uses, uses, active, expires_at FROM invites WHERE code=$1`, req.InviteCode).Scan(&inviteID, &maxUses, &uses, &active, &expiresAt)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid invite"})
			return
		}
		if !active || (expiresAt.Valid && time.Now().After(expiresAt.Time)) || uses >= maxUses {
			c.JSON(http.StatusForbidden, gin.H{"error": "invite not usable"})
			return
		}
		pwdHash, err := auth.HashPassword(req.Password)
		if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "hash error"}); return }
		pinHash, err := auth.HashPassword(req.PIN)
		if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "hash error"}); return }
		var userID uuid.UUID
		err = db.QueryRow(`INSERT INTO users (username, display_name, password_hash, pin_hash, public_key) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
			req.Username, req.DisplayName, pwdHash, pinHash, req.PublicKey).Scan(&userID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username taken?"})
			return
		}
		_, _ = db.Exec(`UPDATE invites SET uses = uses + 1 WHERE id=$1`, inviteID)
		token, err := auth.GenerateJWT(cfg.JWTSecret, userID.String(), req.Username, 24*time.Hour)
		if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "token error"}); return }
		c.JSON(http.StatusOK, gin.H{"token": token, "userId": userID.String()})
	})

	r.POST("/login", func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var id, username, pwdHash, pinHash string
		err := db.QueryRow(`SELECT id, username, password_hash, pin_hash FROM users WHERE username=$1`, req.Username).Scan(&id, &username, &pwdHash, &pinHash)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		ok, _ := auth.VerifyPassword(pwdHash, req.Password)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		ok, _ = auth.VerifyPassword(pinHash, req.PIN)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		token, err := auth.GenerateJWT(cfg.JWTSecret, id, username, 24*time.Hour)
		if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "token error"}); return }
		c.JSON(http.StatusOK, gin.H{"token": token, "userId": id})
	})
}