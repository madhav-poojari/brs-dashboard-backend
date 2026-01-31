package models

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleCoach   Role = "coach"
	RoleMentor  Role = "mentor"
	RoleStudent Role = "student"
)
