package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// BlogVisibility defines the visibility of a blog post.
type BlogVisibility string

const (
	BlogVisibilityPublic   BlogVisibility = "public"
	BlogVisibilityInternal BlogVisibility = "internal"
)

// BlogStatus defines the publication status of a blog post.
type BlogStatus string

const (
	BlogStatusDraft     BlogStatus = "draft"
	BlogStatusPublished BlogStatus = "published"
)

// Blog represents a blog post.
type Blog struct {
	ID            string         `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Slug          string         `gorm:"uniqueIndex;not null" json:"slug"`
	Title         string         `gorm:"not null" json:"title"`
	Summary       string         `gorm:"type:text" json:"summary"`
	Content       datatypes.JSON `gorm:"type:jsonb;not null" json:"content"` // TipTap JSON stored as JSONB
	CoverImageURL string         `gorm:"column:cover_image_url" json:"cover_image_url"`
	AuthorID      string         `gorm:"size:10;not null;index" json:"author_id"`
	Author        User           `gorm:"foreignKey:AuthorID;references:ID" json:"author,omitempty"`
	Visibility    BlogVisibility `gorm:"type:text;not null;default:'public'" json:"visibility"`
	Status        BlogStatus     `gorm:"type:text;not null;default:'draft';index" json:"status"`
	PublishedAt   *time.Time     `json:"published_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	Tags          []BlogTag      `gorm:"many2many:blog_tag_mappings;joinForeignKey:BlogID;joinReferences:BlogTagID" json:"tags,omitempty"`
	CanEdit       bool           `gorm:"-" json:"can_edit"`
}

// BlogTag represents a tag that can be applied to blogs.
type BlogTag struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Slug      string    `gorm:"uniqueIndex;not null" json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

// BlogTagMapping is the join table between blogs and tags.
type BlogTagMapping struct {
	BlogID    string `gorm:"type:uuid;primaryKey" json:"blog_id"`
	BlogTagID string `gorm:"type:uuid;primaryKey" json:"blog_tag_id"`
}

func (BlogTagMapping) TableName() string { return "blog_tag_mappings" }

// BlogImage represents an image uploaded within a blog post.
type BlogImage struct {
	ID        string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	BlogID    string    `gorm:"type:uuid;not null;index" json:"blog_id"`
	URLSuffix string    `gorm:"column:url_suffix;not null" json:"url_suffix"`
	AltText   string    `json:"alt_text"`
	CreatedAt time.Time `json:"created_at"`
}
