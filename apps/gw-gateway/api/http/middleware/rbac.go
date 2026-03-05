package middleware

import (
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

// RBACMiddleware returns a Gin handler that enforces Casbin domain-scoped RBAC.
// resource and action are the policy objects being checked (e.g. "/tasks", "write").
func RBACMiddleware(enforcer *casbin.Enforcer, resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString(string(ContextKeyRole))
		companyID := c.GetString(string(ContextKeyCompanyID))

		if role == "" || companyID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing auth claims"})
			return
		}

		// Domain-scoped check: (role, company_id, resource, action)
		// Policy uses wildcard domain "*" so we check against the actual company_id.
		// Casbin g() role lookup is domain-scoped via the model.
		allowed, err := enforcer.Enforce(role, companyID, resource, action)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "authorization check failed"})
			return
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}

		c.Next()
	}
}

// NewCasbinEnforcer loads model and policy from files and returns a ready enforcer.
func NewCasbinEnforcer(modelPath, policyPath string) (*casbin.Enforcer, error) {
	return casbin.NewEnforcer(modelPath, policyPath)
}
