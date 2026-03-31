package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/ungweiliang/selfhost-paas/internal/store/generated"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

type AuthService struct {
	queries        *generated.Queries
	jwtSecret      []byte
	jwtExpiryHours int
}

type AuthClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type AuthResult struct {
	Token string         `json:"token"`
	User  AuthUserResult `json:"user"`
}

type AuthUserResult struct {
	ID    pgtype.UUID `json:"id"`
	Email string      `json:"email"`
	Role  string      `json:"role"`
}

func NewAuthService(queries *generated.Queries, jwtSecret string, jwtExpiryHours int) *AuthService {
	if jwtExpiryHours <= 0 {
		jwtExpiryHours = 24
	}
	return &AuthService{
		queries:        queries,
		jwtSecret:      []byte(jwtSecret),
		jwtExpiryHours: jwtExpiryHours,
	}
}

func (s *AuthService) Register(ctx context.Context, email, password, role string) (generated.User, error) {
	// Check if user already exists
	_, err := s.queries.GetUserByEmail(ctx, email)
	if err == nil {
		return generated.User{}, ErrUserAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return generated.User{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.queries.CreateUser(ctx, generated.CreateUserParams{
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
	})
	if err != nil {
		return generated.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &AuthResult{
		Token: token,
		User: AuthUserResult{
			ID:    user.ID,
			Email: user.Email,
			Role:  user.Role,
		},
	}, nil
}

func (s *AuthService) ValidateToken(tokenString string) (*AuthClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AuthClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*AuthClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (s *AuthService) IsSetupRequired(ctx context.Context) (bool, error) {
	count, err := s.queries.CountUsers(ctx)
	if err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return count == 0, nil
}

func (s *AuthService) generateToken(user generated.User) (string, error) {
	// Convert pgtype.UUID to string
	ub := user.ID.Bytes
	userIDStr := fmt.Sprintf("%x-%x-%x-%x-%x", ub[0:4], ub[4:6], ub[6:8], ub[8:10], ub[10:16])

	claims := AuthClaims{
		UserID: userIDStr,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(s.jwtExpiryHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userIDStr,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}
