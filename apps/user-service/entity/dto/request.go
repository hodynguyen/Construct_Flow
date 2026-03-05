package dto

// RegisterRequest is the input for user registration.
type RegisterRequest struct {
	Email       string `json:"email"`
	Name        string `json:"name"`
	Password    string `json:"password"`
	Role        string `json:"role"`        // admin | manager | worker
	CompanyID   string `json:"company_id"`  // join existing company (mutually exclusive with CompanyName)
	CompanyName string `json:"company_name"` // create new company
}

// LoginRequest is the input for authentication.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
