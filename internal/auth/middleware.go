package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

// UserService interface for querying user data
type UserService interface {
	GetUserRole(userID string) (string, error)
}

type UserClaims struct {
	UserID     string   `json:"user_id"`
	Sub        string   `json:"sub"` // NestJS often uses 'sub' for user ID
	ID         int      `json:"id"`  // Alternative ID field
	Email      string   `json:"email"`
	Username   string   `json:"username"` // NestJS might use username
	PropertyID *int64   `json:"property_id,omitempty"`
	Role       string   `json:"role"`
	Roles      interface{} `json:"roles"` // Can be number, string, or array
	jwt.StandardClaims
}

type ContextKey string

const (
	UserContextKey ContextKey = "user"
	JWTSecretKey   string     = "JWT_SECRET" // Should be in env
)

// AuthMiddleware validates JWT tokens from the main NestJS backend
func AuthMiddleware(jwtSecret string, userService UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		println(authHeader)
		println(jwtSecret)

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

		token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid tokens bibash"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
			// Normalize user ID from different claim formats
			userID := claims.UserID
			if userID == "" && claims.Sub != "" {
				userID = claims.Sub
			}
			if userID == "" && claims.ID != 0 {
				userID = fmt.Sprintf("%d", claims.ID)
			}

			// For TheNimto backend, roles are not in JWT token
			// Try to get role from custom header first, then from JWT claims
			role := c.GetHeader("X-User-Role")
			if role == "" {
				role = claims.Role
				if role == "" {
					// Handle roles field which can be number, string, or array
					switch v := claims.Roles.(type) {
					case float64:
						role = fmt.Sprintf("%.0f", v)
					case int:
						role = fmt.Sprintf("%d", v)
					case string:
						role = v
					case []interface{}:
						if len(v) > 0 {
							if str, ok := v[0].(string); ok {
								role = str
							} else if num, ok := v[0].(float64); ok {
								role = fmt.Sprintf("%.0f", num)
							}
						}
					case []string:
						if len(v) > 0 {
							role = v[0]
						}
					}
				}
			}
			
			// If role is still empty, look it up from the database
			if role == "" && userID != "" {
				// Query the main database for user role
				dbRole, err := userService.GetUserRole(userID)
				if err != nil {
					println("Failed to get user role from database:", err.Error())
					// Continue without role - let the handlers decide access
				} else {
					role = dbRole
					println("Retrieved role from database for user", userID+":", role)
				}
			}
			
			// Store user info in context
			c.Set("user", claims)
			c.Set("user_id", userID)
			c.Set("property_id", claims.PropertyID)
			c.Set("role", role)
			c.Set("email", claims.Email)
			if claims.Username != "" {
				c.Set("username", claims.Username)
			}
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}
	}
}

// OptionalAuthMiddleware allows both authenticated and unauthenticated requests
func OptionalAuthMiddleware(jwtSecret string, userService UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)

		token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})

		if err == nil && token.Valid {
			if claims, ok := token.Claims.(*UserClaims); ok {
				// Normalize user ID from different claim formats
				userID := claims.UserID
				if userID == "" && claims.Sub != "" {
					userID = claims.Sub
				}
				if userID == "" && claims.ID != 0 {
					userID = fmt.Sprintf("%d", claims.ID)
				}

				// Normalize role from different formats
				role := claims.Role
				if role == "" {
					// Handle roles field which can be number, string, or array
					switch v := claims.Roles.(type) {
					case float64:
						role = fmt.Sprintf("%.0f", v)
					case int:
						role = fmt.Sprintf("%d", v)
					case string:
						role = v
					case []interface{}:
						if len(v) > 0 {
							if str, ok := v[0].(string); ok {
								role = str
							} else if num, ok := v[0].(float64); ok {
								role = fmt.Sprintf("%.0f", num)
							}
						}
					case []string:
						if len(v) > 0 {
							role = v[0]
						}
					}
				}

				// If role is still empty, look it up from the database
				if role == "" && userID != "" {
					// Query the main database for user role
					dbRole, err := userService.GetUserRole(userID)
					if err != nil {
						println("Failed to get user role from database:", err.Error())
						// Continue without role - let the handlers decide access
					} else {
						role = dbRole
						println("Retrieved role from database for user", userID+":", role)
					}
				}

				c.Set("user", claims)
				c.Set("user_id", userID)
				c.Set("property_id", claims.PropertyID)
				c.Set("role", role)
				c.Set("email", claims.Email)
				if claims.Username != "" {
					c.Set("username", claims.Username)
				}
			}
		}

		c.Next()
	}
}

// GetUserFromContext extracts user claims from gin context
func GetUserFromContext(c *gin.Context) (*UserClaims, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	claims, ok := user.(*UserClaims)
	return claims, ok
}

// ValidateMainBackendToken validates token from main NestJS backend
func ValidateMainBackendToken(token string, mainBackendURL string, jwtSecret string) (*UserClaims, error) {
	// In production, this would validate against the main backend
	// For now, we'll parse the JWT directly
	claims := &UserClaims{}

	parsedToken, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		// Get secret from environment or config
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// GetUserContext creates a context with user information
func GetUserContext(ctx context.Context, userClaims *UserClaims) context.Context {
	return context.WithValue(ctx, UserContextKey, userClaims)
}
