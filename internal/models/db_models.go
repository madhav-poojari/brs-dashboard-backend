package models

import "time"
import "gorm.io/datatypes"

type User struct {
	ID    string `gorm:"primaryKey;size:10" json:"id"`
	Email string `gorm:"uniqueIndex;not null" json:"email"`

	PasswordHash string      `json:"-"`
	FirstName    string      `json:"first_name"`
	LastName     string      `json:"last_name"`
	Role         Role        `gorm:"type:text;not null" json:"role"`
	Approved     bool        `gorm:"default:false" json:"approved"`
	Active       bool        `gorm:"default:true" json:"active"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	UserDetails  UserDetails `gorm:"foreignKey:UserID" json:"details,omitempty"`
}

type UserDetails struct {
	UserID            string            `gorm:"primaryKey;size:10" json:"user_id"`
	City              string            `json:"city"`
	State             string            `json:"state"`
	Country           string            `json:"country"`
	Zipcode           string            `json:"zipcode"`
	Phone             string            `json:"phone"`
	DOB               *time.Time        `json:"dob"`
	LichessUsername   string            `json:"lichess_username"`
	USCFID            string            `json:"uscf_id"`
	ChesscomUsername  string            `json:"chesscom_username"`
	FIDEID            string            `json:"fide_id"`
	Bio               string            `json:"bio"`
	ProfilePictureURL string            `json:"profile_picture_url"`
	AdditionalInfo    datatypes.JSONMap `gorm:"type:jsonb" json:"additional_info"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type RefreshToken struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID    string    `gorm:"index;size:10" json:"user_id"`
	TokenHash string    `gorm:"not null" json:"-"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `gorm:"default:false" json:"revoked"`
}

type CoachStudent struct {
	CoachID       string `gorm:"size:10;primaryKey"`
	StudentID     string `gorm:"size:10;primaryKey"`
	MentorCoachID string `gorm:"size:10;index"` // added column
}
