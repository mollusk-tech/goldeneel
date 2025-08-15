package routes

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"messagingapi/internal/config"
)

type createInviteRequest struct {
	Code     string     `json:"code"`
	MaxUses  int        `json:"maxUses"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

func RegisterInviteRoutes(r *gin.RouterGroup, db *sql.DB, cfg config.Config) {
	r.POST("/", func(c *gin.Context) {
		uid := c.GetString("userID")
		var isAdmin bool
		if err := db.QueryRow(`SELECT is_admin FROM users WHERE id=$1`, uid).Scan(&isAdmin); err != nil || !isAdmin { c.JSON(http.StatusForbidden, gin.H{"error":"admin only"}); return }
		var req createInviteRequest
		if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
		if req.MaxUses <= 0 { req.MaxUses = 1 }
		code := req.Code
		if code == "" { code = uuid.NewString() }
		_, err := db.Exec(`INSERT INTO invites (code, created_by, max_uses, uses, active, expires_at) VALUES ($1,$2,$3,0,true,$4)`, code, uid, req.MaxUses, req.ExpiresAt)
		if err != nil { c.JSON(http.StatusBadRequest, gin.H{"error":"duplicate code?"}); return }
		c.JSON(http.StatusOK, gin.H{"code": code})
	})

	r.GET("/", func(c *gin.Context) {
		uid := c.GetString("userID")
		var isAdmin bool
		if err := db.QueryRow(`SELECT is_admin FROM users WHERE id=$1`, uid).Scan(&isAdmin); err != nil || !isAdmin { c.JSON(http.StatusForbidden, gin.H{"error":"admin only"}); return }
		rows, err := db.Query(`SELECT code, max_uses, uses, active, expires_at FROM invites ORDER BY created_at DESC`)
		if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error":"db"}); return }
		defer rows.Close()
		var list []gin.H
		for rows.Next() {
			var code string
			var maxUses, uses int
			var active bool
			var expiresAt sql.NullTime
			if err := rows.Scan(&code, &maxUses, &uses, &active, &expiresAt); err == nil {
				item := gin.H{"code": code, "maxUses": maxUses, "uses": uses, "active": active}
				if expiresAt.Valid { item["expiresAt"] = expiresAt.Time }
				list = append(list, item)
			}
		}
		c.JSON(http.StatusOK, gin.H{"invites": list})
	})
}