package domain

// TokenClaims contains the data embedded in a JWT token.
type TokenClaims struct {
	UserID    string
	CompanyID string
	Role      string
}

// TokenService handles JWT generation and validation.
// The domain layer depends only on this interface — never on the jwt library directly.
type TokenService interface {
	GenerateToken(claims TokenClaims) (string, error)
	ValidateToken(token string) (*TokenClaims, error)
}
