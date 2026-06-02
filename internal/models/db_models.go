package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type User struct {
	ID    string `gorm:"primaryKey;size:10" json:"id"`
	Email string `gorm:"unique;not null" json:"email"`

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
	USCFID            string            `gorm:"column:uscf_id" json:"uscf_id"`
	ChesscomUsername  string            `json:"chesscom_username"`
	FIDEID            string            `gorm:"column:fide_id" json:"fide_id"`
	Bio               string            `json:"bio"`
	ProfilePictureURL string            `json:"profile_picture_url"`
	SyllabusURL       string            `json:"syllabus_url"`
	AddedInWhatsapp   bool              `gorm:"default:false" json:"added_in_whatsapp"`
	PersonalMeetLink  string            `json:"personal_meet_link"`
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

// Relation: table "relations", columns user_id (PK), coach_id, mentor_id.
// For students:  user_id = student_id, coach_id = assigned coach, mentor_id = assigned mentor.
// For coaches:   user_id = coach_id,   coach_id = "" (empty),     mentor_id = assigned mentor.
// One row per user_id.
type Relation struct {
	UserID   string `gorm:"column:user_id;size:10;primaryKey"`
	CoachID  string `gorm:"column:coach_id;size:10;index"`
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

type Image struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    string         `gorm:"index;size:10;not null" json:"user_id"`
	URLSuffix string         `gorm:"column:url_suffix;not null" json:"url_suffix"`
	Title     string         `gorm:"not null" json:"title"`
	Tags      datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"tags"`
	IsPrivate bool           `gorm:"default:false" json:"is_private"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Filename  string         `gorm:"null" json:"filename"`
}

type ZipcodeScrapeScope struct {
	Zipcode     string     `gorm:"primaryKey;size:10" json:"zipcode"`
	Distance    int        `gorm:"primaryKey" json:"distance"`
	LastScraped *time.Time `json:"last_scraped"`
}

func (ZipcodeScrapeScope) TableName() string { return "zipcode_scrape_scopes" }

type Tournament struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Title       string     `json:"title"`
	URLPath     string     `gorm:"column:url_path;uniqueIndex" json:"url_path"`
	City        string     `json:"city"`
	State       string     `json:"state"`
	Dates       string     `json:"dates"`
	StartDate   *time.Time `gorm:"type:date" json:"start_date"`
	Organizer   string     `json:"organizer"`
	Description string     `gorm:"type:text" json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
}

type TournamentWithinRadius struct {
	Zipcode      string `gorm:"primaryKey;size:10" json:"zipcode"`
	Distance     int    `gorm:"primaryKey" json:"distance"`
	TournamentID uint   `gorm:"primaryKey" json:"tournament_id"`
}

func (TournamentWithinRadius) TableName() string { return "tournaments_within_radius" }

type ClassSchedule struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	StudentID string `gorm:"index;size:10;not null" json:"student_id"`
	Student   User   `gorm:"foreignKey:StudentID;references:ID" json:"student,omitempty"`
	DayOfWeek int    `gorm:"not null" json:"day_of_week"`          // 0=Sun..6=Sat
	StartTime string `gorm:"type:text;not null" json:"start_time"` // "HH:MM" in student's timezone
	Timezone  string `gorm:"type:text;not null" json:"timezone"`   // IANA timezone e.g. "America/New_York"
}

type ReferralRelationship struct {
	ID                      string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ReferrerID              string `gorm:"size:10;index"`
	RefereeID               string `gorm:"size:10;index"`
	RelationshipType        string `gorm:"index"`
	RelationshipDescription *string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	CreatedBy               *string `gorm:"size:10"`
}

func (ReferralRelationship) TableName() string {
	return "referral_relationships"                        // IANA timezone e.g. "America/New_York"
}

type RatingHistory struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     string    `gorm:"index;size:10;not null" json:"user_id"`
	Platform   string    `gorm:"type:text;not null" json:"platform"`    // chesscom, lichess, fide, uscf
	RatingType string    `gorm:"type:text;not null" json:"rating_type"` // rapid, classical
	Rating     int       `gorm:"not null" json:"rating"`
	RecordedAt time.Time `gorm:"not null" json:"recorded_at"`
	CreatedAt  time.Time `json:"created_at"`
}
