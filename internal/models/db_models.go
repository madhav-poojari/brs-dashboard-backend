package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

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

// Relation: table "relations", columns user_id, coach_id, mentor_id.
// Same shape as old coach_students (student_id→user_id, mentor_coach_id→mentor_id). Composite PK (coach_id, user_id).
// TableName() tells GORM the table is "relations" instead of inferring from the struct name.
type Relation struct {
	CoachID  string `gorm:"column:coach_id;size:10;primaryKey"`
	UserID   string `gorm:"column:user_id;size:10;primaryKey"`
	MentorID string `gorm:"column:mentor_id;size:10;index"`
}

func (Relation) TableName() string { return "relations" }

type LessonPlan struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	UserID      string         `gorm:"index;size:10" json:"user_id"`
	Title       string         `json:"title"`
	Description datatypes.JSON `gorm:"type:jsonb" json:"description"` // array of strings
	StartDate   time.Time      `json:"start_date"`
	EndDate     time.Time      `json:"end_date"`
	Result      string         `json:"result"`
	Active      bool           `gorm:"default:true;index" json:"active"`
	CreatedBy   string         `gorm:"size:10" json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type Note struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID string `gorm:"index;size:10" json:"user_id"`
	// StudentID      string         `gorm:"index;size:10" json:"student_id"`
	Title          string         `json:"title"`
	Description    string         `gorm:"type:text" json:"description"`
	PrimaryTag     string         `json:"primary_tag"`
	Tags           datatypes.JSON `gorm:"type:jsonb" json:"tags"` // JSON array
	IsStarred      bool           `gorm:"default:false" json:"is_starred"`
	AdditionalInfo datatypes.JSON `gorm:"type:jsonb" json:"additional_info"`
	Visibility     int            `gorm:"not null" json:"visibility"`
	CreatedBy      string         `gorm:"size:10" json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

type AttendanceClassType string

const (
	AttendanceClassTypeRegular      AttendanceClassType = "regular"
	AttendanceClassTypeGameSession  AttendanceClassType = "game_session"
	AttendanceClassTypeDual         AttendanceClassType = "dual"
	AttendanceClassTypeSubstitution AttendanceClassType = "substitution"
)

type Attendance struct {
	ID uint `gorm:"primaryKey" json:"id"`

	StudentID string `gorm:"index;size:10;not null" json:"student_id"`
	Student   User   `gorm:"foreignKey:StudentID;references:ID" json:"student,omitempty"`

	CoachID string `gorm:"index;size:10;not null" json:"coach_id"`
	Coach   User   `gorm:"foreignKey:CoachID;references:ID" json:"coach,omitempty"`

	ClassType AttendanceClassType `gorm:"type:text;not null" json:"class_type"`
	Date      time.Time           `gorm:"type:date;index;not null" json:"date"`
	SessionID string              `gorm:"index;size:64" json:"session_id,omitempty"`

	IsVerified      bool   `gorm:"default:false" json:"is_verified"`
	ClassHighlights string `gorm:"type:text" json:"class_highlights"`
	Homework        string `gorm:"type:text" json:"homework"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
