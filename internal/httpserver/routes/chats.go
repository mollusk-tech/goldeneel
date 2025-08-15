package routes

import (
	"database/sql"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"messagingapi/internal/config"
)

type createChatRequest struct {
	Title    string   `json:"title"`
	IsGroup  bool     `json:"isGroup"`
	MemberIDs []string `json:"memberIds" binding:"required"`
}

func RegisterChatRoutes(r *gin.RouterGroup, db *sql.DB, cfg config.Config) {
	r.POST("/", func(c *gin.Context) {
		uid := c.GetString("userID")
		var req createChatRequest
		if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
		var chatID uuid.UUID
		err := db.QueryRow(`INSERT INTO chats (title, is_group, created_by) VALUES ($1,$2,$3) RETURNING id`, req.Title, req.IsGroup, uid).Scan(&chatID)
		if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error":"create chat"}); return }
		_, _ = db.Exec(`INSERT INTO chat_members (chat_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, chatID, uid)
		for _, mid := range req.MemberIDs {
			_, _ = db.Exec(`INSERT INTO chat_members (chat_id, user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, chatID, mid)
		}
		c.JSON(http.StatusOK, gin.H{"chatId": chatID.String()})
	})

	r.DELETE("/:id/clear", func(c *gin.Context) {
		uid := c.GetString("userID")
		chatID := c.Param("id")
		var member int
		if err := db.QueryRow(`SELECT COUNT(*) FROM chat_members WHERE chat_id=$1 AND user_id=$2`, chatID, uid).Scan(&member); err != nil || member == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error":"not member"}); return
		}
		// Delete attachments files and messages in this chat
		rows, err := db.Query(`SELECT a.file_path FROM attachments a JOIN messages m ON a.message_id=m.id WHERE m.chat_id=$1`, chatID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var p string
				if err := rows.Scan(&p); err == nil { _ = os.Remove(p) }
			}
		}
		_, _ = db.Exec(`DELETE FROM messages WHERE chat_id=$1`, chatID)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}