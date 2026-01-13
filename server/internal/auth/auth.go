package auth

import (
	"fmt"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sp00nznet/octopus/internal/config"
)

// Authenticator handles user authentication via AD
type Authenticator struct {
	config *config.Config
}

// User represents an authenticated user
type User struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	IsAdmin     bool   `json:"is_admin"`
}

// Claims represents JWT claims
type Claims struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// New creates a new authenticator
func New(cfg *config.Config) *Authenticator {
	return &Authenticator{config: cfg}
}

// Authenticate validates credentials against AD and returns a JWT token
func (a *Authenticator) Authenticate(username, password string) (*User, string, error) {
	// If AD is not configured, use local auth (for development)
	if a.config.ADServer == "" {
		return a.localAuth(username, password)
	}

	// Connect to AD server
	l, err := ldap.DialURL(fmt.Sprintf("ldap://%s:389", a.config.ADServer))
	if err != nil {
		return nil, "", fmt.Errorf("failed to connect to AD: %w", err)
	}
	defer l.Close()

	// Bind with service account to search
	err = l.Bind(a.config.ADBindUser, a.config.ADBindPass)
	if err != nil {
		return nil, "", fmt.Errorf("failed to bind to AD: %w", err)
	}

	// Search for user
	searchFilter := fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", ldap.EscapeFilter(username))
	searchRequest := ldap.NewSearchRequest(
		a.config.ADBaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		searchFilter,
		[]string{"dn", "cn", "mail", "memberOf"},
		nil,
	)

	result, err := l.Search(searchRequest)
	if err != nil {
		return nil, "", fmt.Errorf("AD search failed: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, "", fmt.Errorf("user not found")
	}

	userDN := result.Entries[0].DN
	displayName := result.Entries[0].GetAttributeValue("cn")
	email := result.Entries[0].GetAttributeValue("mail")
	memberOf := result.Entries[0].GetAttributeValues("memberOf")

	// Verify user credentials
	err = l.Bind(userDN, password)
	if err != nil {
		return nil, "", fmt.Errorf("invalid credentials")
	}

	// Check if user is in admin group
	isAdmin := false
	for _, group := range memberOf {
		if containsAdminGroup(group) {
			isAdmin = true
			break
		}
	}

	user := &User{
		Username:    username,
		DisplayName: displayName,
		Email:       email,
		IsAdmin:     isAdmin,
	}

	// Generate JWT token
	token, err := a.generateToken(user)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

// localAuth provides local authentication for development
func (a *Authenticator) localAuth(username, password string) (*User, string, error) {
	// For development: accept admin/admin
	if username == "admin" && password == "admin" {
		user := &User{
			Username:    "admin",
			DisplayName: "Administrator",
			Email:       "admin@localhost",
			IsAdmin:     true,
		}
		token, err := a.generateToken(user)
		return user, token, err
	}

	// Accept any user/user combo in dev mode
	if username == password && username != "" {
		user := &User{
			Username:    username,
			DisplayName: username,
			Email:       username + "@localhost",
			IsAdmin:     false,
		}
		token, err := a.generateToken(user)
		return user, token, err
	}

	return nil, "", fmt.Errorf("invalid credentials")
}

// generateToken creates a JWT token for the user
func (a *Authenticator) generateToken(user *User) (string, error) {
	expirationTime := time.Now().Add(time.Duration(a.config.JWTExpiration) * time.Hour)

	claims := &Claims{
		Username: user.Username,
		IsAdmin:  user.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "octopus",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.config.JWTSecret))
}

// ValidateToken validates a JWT token and returns the claims
func (a *Authenticator) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// containsAdminGroup checks if the group DN contains an admin group
func containsAdminGroup(groupDN string) bool {
	// Check for common admin group names
	adminGroups := []string{
		"Domain Admins",
		"Octopus Admins",
		"CN=Administrators",
	}

	for _, adminGroup := range adminGroups {
		if ldap.EscapeFilter(groupDN) != groupDN {
			continue
		}
		if containsIgnoreCase(groupDN, adminGroup) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsIgnoreCase(s[1:], substr))
}
