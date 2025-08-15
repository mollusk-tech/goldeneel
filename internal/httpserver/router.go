package httpserver

import (
	"crypto/tls"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"messagingapi/internal/auth"
	"messagingapi/internal/config"
	"messagingapi/internal/httpserver/routes"
)

type contextKey string

func NewRouter(db *sql.DB, cfg config.Config) *gin.Engine {
	if gin.Mode() == gin.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())
	r.Use(lastActiveMiddleware(db))
	r.Use(corsMiddleware())

	api := r.Group("/api/v1")

	routes.RegisterAuthRoutes(api.Group("/auth"), db, cfg)
	authRequired := api.Group("")
	authRequired.Use(jwtAuthMiddleware(cfg.JWTSecret))
	routes.RegisterUserRoutes(authRequired.Group("/users"), db, cfg)
	routes.RegisterChatRoutes(authRequired.Group("/chats"), db, cfg)
	routes.RegisterMessageRoutes(authRequired.Group("/messages"), db, cfg)
	routes.RegisterMediaRoutes(authRequired.Group("/media"), db, cfg)
	routes.RegisterInviteRoutes(authRequired.Group("/invites"), db, cfg)

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	return r
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		_ = latency
		_ = status
		_ = method
		_ = path
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func jwtAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authz := c.GetHeader("Authorization")
		if !strings.HasPrefix(authz, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		t := strings.TrimPrefix(authz, "Bearer ")
		claims, err := auth.ParseJWT(secret, t)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

func lastActiveMiddleware(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if uid, ok := c.Get("userID"); ok {
			_, _ = db.Exec(`UPDATE users SET last_active_at=now() WHERE id=$1`, uid)
		}
	}
}

func ListenAndServeTLS(handler http.Handler, addr, certFile, keyFile string) error {
	server := &http.Server{Addr: addr, Handler: handler}
	server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return server.ListenAndServeTLS(certFile, keyFile)
}