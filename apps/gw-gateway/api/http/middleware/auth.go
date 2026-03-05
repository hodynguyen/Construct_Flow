package middleware

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// contextKey type avoids key collisions with other packages.
type contextKey string

const (
	ContextKeyUserID    contextKey = "user_id"
	ContextKeyCompanyID contextKey = "company_id"
	ContextKeyRole      contextKey = "role"
)

type jwtClaims struct {
	jwt.RegisteredClaims
	UserID    string `json:"user_id"`
	CompanyID string `json:"company_id"`
	Role      string `json:"role"`
}

// AuthMiddleware validates RS256 JWT tokens and injects claims into the gin context.
func AuthMiddleware(publicKeyPath string) (gin.HandlerFunc, error) {
	pubBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading JWT public key: %w", err)
	}
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing JWT public key: %w", err)
	}

	return authHandler(pubKey), nil
}

func authHandler(pubKey *rsa.PublicKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed Authorization header"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return pubKey, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(*jwtClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "malformed token claims"})
			return
		}
		if claims.UserID == "" || claims.CompanyID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "incomplete token claims"})
			return
		}

		c.Set(string(ContextKeyUserID), claims.UserID)
		c.Set(string(ContextKeyCompanyID), claims.CompanyID)
		c.Set(string(ContextKeyRole), claims.Role)
		c.Next()
	}
}

// RequireRoles returns 403 if the caller's role is not in the allowed list.
func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		role := c.GetString(string(ContextKeyRole))
		if !allowed[role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

// GetClaims extracts JWT claims from the gin context (panics if auth middleware not used).
func GetClaims(c *gin.Context) (userID, companyID, role string, err error) {
	userID = c.GetString(string(ContextKeyUserID))
	companyID = c.GetString(string(ContextKeyCompanyID))
	role = c.GetString(string(ContextKeyRole))
	if userID == "" || companyID == "" {
		err = errors.New("auth claims not found in context")
	}
	return
}
