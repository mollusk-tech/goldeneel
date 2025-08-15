package routes

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"messagingapi/internal/config"
)

type sendMessageRequest struct {
	ChatID   string `form:"chatId" binding:"required"`
	Cipher   string `form:"ciphertext" binding:"required"`
	Nonce    string `form:"nonce"`
	ReplyTo  string `form:"replyToId"`
}

type editMessageRequest struct {
	Cipher string `json:"ciphertext" binding:"required"`
	Nonce  string `json:"nonce"`
}

func RegisterMessageRoutes(r *gin.RouterGroup, db *sql.DB, cfg config.Config) {
	r.POST("/", func(c *gin.Context) {
		uid := c.GetString("userID")
		var req sendMessageRequest
		if err := c.ShouldBind(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
		// Ensure membership
		var member int
		if err := db.QueryRow(`SELECT COUNT(*) FROM chat_members WHERE chat_id=$1 AND user_id=$2`, req.ChatID, uid).Scan(&member); err != nil || member == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error":"not member"}); return
		}
		var replyTo *uuid.UUID
		if req.ReplyTo != "" { if id, err := uuid.Parse(req.ReplyTo); err == nil { replyTo = &id } }
		var msgID uuid.UUID
		err := db.QueryRow(`INSERT INTO messages (chat_id, sender_id, ciphertext, nonce, reply_to) VALUES ($1,$2,$3,$4,$5) RETURNING id`, req.ChatID, uid, req.Cipher, req.Nonce, replyTo).Scan(&msgID)
		if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error":"create message"}); return }
		form, _ := c.MultipartForm()
		if form != nil {
			files := form.File["files"]
			for _, f := range files {
				dir := filepath.Join(cfg.DataDir, "uploads", req.ChatID)
				_ = os.MkdirAll(dir, 0755)
				name := fmt.Sprintf("%s-%d%s", msgID.String(), time.Now().UnixNano(), filepath.Ext(f.Filename))
				path := filepath.Join(dir, name)
				if err := c.SaveUploadedFile(f, path); err == nil {
					_, _ = db.Exec(`INSERT INTO attachments (message_id, file_path, content_type, size_bytes) VALUES ($1,$2,$3,$4)`, msgID, path, f.Header.Get("Content-Type"), f.Size)
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"messageId": msgID.String()})
	})

	r.PATCH("/:id", func(c *gin.Context) {
		uid := c.GetString("userID")
		id := c.Param("id")
		var req editMessageRequest
		if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
		var sender string
		if err := db.QueryRow(`SELECT sender_id FROM messages WHERE id=$1`, id).Scan(&sender); err != nil { c.JSON(http.StatusNotFound, gin.H{"error":"not found"}); return }
		if sender != uid { c.JSON(http.StatusForbidden, gin.H{"error":"not owner"}); return }
		_, err := db.Exec(`UPDATE messages SET ciphertext=$1, nonce=$2, edited_at=now() WHERE id=$3`, req.Cipher, req.Nonce, id)
		if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error":"update"}); return }
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	r.DELETE("/:id", func(c *gin.Context) {
		uid := c.GetString("userID")
		id := c.Param("id")
		var sender string
		if err := db.QueryRow(`SELECT sender_id FROM messages WHERE id=$1`, id).Scan(&sender); err != nil { c.JSON(http.StatusNotFound, gin.H{"error":"not found"}); return }
		if sender != uid { c.JSON(http.StatusForbidden, gin.H{"error":"not owner"}); return }
		// Delete attachments files
		rows, _ := db.Query(`SELECT file_path FROM attachments WHERE message_id=$1`, id)
		if rows != nil { defer rows.Close(); for rows.Next() { var p string; if err := rows.Scan(&p); err == nil { _ = os.Remove(p) } } }
		_, _ = db.Exec(`DELETE FROM messages WHERE id=$1`, id)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}