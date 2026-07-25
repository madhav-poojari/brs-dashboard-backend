package models

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type PersonInfo struct {
	Name  string `json:"name"`
	ProfilePictureURL string `json:"profile_picture_url,omitempty"`
	FIDEID	 string `json:"fide_id,omitempty"`
	Bio		string `json:"bio"`
	PersonalMeetLink string `json:"personal_meet_link,omitempty"`
}

type UserResponse struct {
	*User
	Mentor   *PersonInfo      `json:"mentor,omitempty"`
	Coach    *PersonInfo      `json:"coach,omitempty"`
	Schedule []*ClassSchedule `json:"schedule,omitempty"`
}

// StudentWithRelations wraps a User with their assigned coach/mentor info.
// Used by the GET /users/ endpoint so coach/mentor callers can group students.
type StudentWithRelations struct {
	*User
	CoachID    string `json:"coach_id,omitempty"`
	CoachName  string `json:"coach_name,omitempty"`
	MentorID   string `json:"mentor_id,omitempty"`
	MentorName string `json:"mentor_name,omitempty"`
}

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleCoach   Role = "coach"
	RoleMentor  Role = "mentor"
	RoleStudent Role = "student"
)

// CreateRelationshipRequest is the request body for creating a relationship
type CreateRelationshipRequest struct {
	ReferrerID              string  `json:"referrer_id"`
	RefereeID               string  `json:"referee_id"`
	RelationshipType        string  `json:"relationship_type"`
	RelationshipDescription *string `json:"relationship_description"`
}

// UpdateRelationshipRequest is the request body for updating a relationship
type UpdateRelationshipRequest struct {
	RelationshipType        *string `json:"relationship_type"`
	RelationshipDescription *string `json:"relationship_description"`
}
