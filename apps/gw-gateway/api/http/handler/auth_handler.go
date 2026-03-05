package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	userv1 "github.com/hodynguyen/construct-flow/gen/go/proto/user_service/v1"
)

// AuthHandler handles public auth endpoints (register, login).
type AuthHandler struct {
	userClient userv1.UserServiceClient
}

func NewAuthHandler(userClient userv1.UserServiceClient) *AuthHandler {
	return &AuthHandler{userClient: userClient}
}

type registerRequest struct {
	Email       string `json:"email"       binding:"required,email"`
	Name        string `json:"name"        binding:"required"`
	Password    string `json:"password"    binding:"required,min=6"`
	Role        string `json:"role"`
	CompanyID   string `json:"company_id"`
	CompanyName string `json:"company_name"`
}

// Register godoc
// @Summary Register a new user
// @Tags auth
// @Accept json
// @Produce json
// @Param body body registerRequest true "Registration payload"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.userClient.Register(c.Request.Context(), &userv1.RegisterRequest{
		Email:       req.Email,
		Name:        req.Name,
		Password:    req.Password,
		Role:        req.Role,
		CompanyId:   req.CompanyID,
		CompanyName: req.CompanyName,
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user":       resp.User,
		"company_id": resp.CompanyId,
	})
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login godoc
// @Summary Authenticate and receive JWT
// @Tags auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "Login payload"
// @Success 200 {object} map[string]interface{}
// @Failure 400,401 {object} map[string]string
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.userClient.Login(c.Request.Context(), &userv1.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		c.JSON(grpcToHTTPStatus(err), gin.H{"error": grpcMessage(err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": resp.AccessToken,
		"user":         resp.User,
	})
}

// grpcToHTTPStatus maps gRPC status codes to HTTP status codes.
func grpcToHTTPStatus(err error) int {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError
	}
	switch st.Code() {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.NotFound:
		return http.StatusNotFound
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

func grpcMessage(err error) string {
	if st, ok := status.FromError(err); ok {
		return st.Message()
	}
	return "internal error"
}
