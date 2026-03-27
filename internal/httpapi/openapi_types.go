package httpapi

import (
	"time"
)

type errorResponse struct {
	Code    string `json:"code" example:"bad_request"`
	Message string `json:"message" example:"invalid payload"`
}

type healthResponse struct {
	Status string `json:"status" example:"ok"`
}

type authUserResponse struct {
	ID        string    `json:"id" example:"user-1"`
	Username  string    `json:"username" example:"admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type authSessionResponse struct {
	ID         string    `json:"id" example:"session-1"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type authLoginResponse struct {
	Token     string               `json:"token" example:"token"`
	ExpiresAt time.Time            `json:"expires_at"`
	User      authUserResponse     `json:"user"`
	Session   authSessionResponse  `json:"session"`
}

type authMeResponse struct {
	User    authUserResponse    `json:"user"`
	Session authSessionResponse `json:"session"`
}
