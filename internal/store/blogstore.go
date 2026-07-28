package store

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/madhava-poojari/dashboard-api/internal/models"
	"gorm.io/gorm"
)

// ─── Slug helpers ───────────────────────────────────────────────────────────────

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a title into a URL-friendly slug.
func slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "blog"
	}
	return s
}

// uniqueSlug generates a slug that doesn't collide with existing ones.
func (s *Store) uniqueSlug(ctx context.Context, title string) (string, error) {
	base := slugify(title)
	slug := base
	for attempt := 0; attempt < 10; attempt++ {
		var count int64
		if err := s.DB.WithContext(ctx).Model(&models.Blog{}).Where("slug = ?", slug).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, time.Now().UnixMilli()%100000)
	}
	return slug, nil
}

// ─── Blog CRUD ──────────────────────────────────────────────────────────────────

// CreateBlog inserts a new blog and syncs its tags in a single transaction.
func (s *Store) CreateBlog(ctx context.Context, blog *models.Blog, tagNames []string) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		slug, err := s.uniqueSlug(ctx, blog.Title)
		if err != nil {
			return err
		}
		blog.Slug = slug
		blog.CreatedAt = time.Now()
		blog.UpdatedAt = time.Now()

		if blog.Status == models.BlogStatusPublished && blog.PublishedAt == nil {
			now := time.Now()
			blog.PublishedAt = &now
		}

		if err := tx.Omit("Tags", "Author").Create(blog).Error; err != nil {
			return err
		}

		if len(tagNames) > 0 {
			return syncBlogTags(tx, ctx, blog.ID, tagNames)
		}
		return nil
	})
}

// GetBlogBySlug fetches a single published blog by its slug (with tags and author).
func (s *Store) GetBlogBySlug(ctx context.Context, slug string) (*models.Blog, error) {
	var blog models.Blog
	if err := s.DB.WithContext(ctx).
		Preload("Tags").
		Preload("Author").
		Where("slug = ?", slug).
		First(&blog).Error; err != nil {
		return nil, err
	}
	return &blog, nil
}

// GetBlogByID fetches a blog by its UUID (with tags and author).
func (s *Store) GetBlogByID(ctx context.Context, id string) (*models.Blog, error) {
	var blog models.Blog
	if err := s.DB.WithContext(ctx).
		Preload("Tags").
		Preload("Author").
		Where("id = ?", id).
		First(&blog).Error; err != nil {
		return nil, err
	}
	return &blog, nil
}

// ListBlogs returns paginated published (non-draft) blogs, ordered by published_at DESC.
// Students only see public blogs; coaches/mentors/admins see both public and internal.
func (s *Store) ListBlogs(ctx context.Context, page, pageSize int, role models.Role) ([]models.Blog, int64, error) {
	var total int64
	q := s.DB.WithContext(ctx).Model(&models.Blog{}).Where("status = ?", models.BlogStatusPublished)
	if role == models.RoleStudent {
		q = q.Where("visibility = ?", models.BlogVisibilityPublic)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var blogs []models.Blog
	offset := (page - 1) * pageSize
	query := s.DB.WithContext(ctx).
		Preload("Tags").
		Preload("Author").
		Where("status = ?", models.BlogStatusPublished)
	if role == models.RoleStudent {
		query = query.Where("visibility = ?", models.BlogVisibilityPublic)
	}
	if err := query.
		Order("published_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&blogs).Error; err != nil {
		return nil, 0, err
	}
	return blogs, total, nil
}

// ListDraftsByAuthor returns draft blogs for a specific author.
func (s *Store) ListDraftsByAuthor(ctx context.Context, authorID string) ([]models.Blog, error) {
	var blogs []models.Blog
	if err := s.DB.WithContext(ctx).
		Preload("Tags").
		Where("author_id = ? AND status = ?", authorID, models.BlogStatusDraft).
		Order("updated_at DESC").
		Find(&blogs).Error; err != nil {
		return nil, err
	}
	return blogs, nil
}

// UpdateBlogFields performs a partial update on a blog.
func (s *Store) UpdateBlogFields(ctx context.Context, blogID string, updates map[string]interface{}, tagNames *[]string) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates["updated_at"] = time.Now()

		// If publishing for the first time, set published_at
		if status, ok := updates["status"]; ok && status == string(models.BlogStatusPublished) {
			var blog models.Blog
			if err := tx.Select("published_at").Where("id = ?", blogID).First(&blog).Error; err != nil {
				return err
			}
			if blog.PublishedAt == nil {
				now := time.Now()
				updates["published_at"] = now
			}
		}

		if err := tx.Model(&models.Blog{}).Where("id = ?", blogID).Updates(updates).Error; err != nil {
			return err
		}

		if tagNames != nil {
			return syncBlogTags(tx, ctx, blogID, *tagNames)
		}
		return nil
	})
}

// DeleteBlogSoft performs a soft delete on a blog.
func (s *Store) DeleteBlogSoft(ctx context.Context, blogID string) error {
	return s.DB.WithContext(ctx).Where("id = ?", blogID).Delete(&models.Blog{}).Error
}

// ─── Tag operations ─────────────────────────────────────────────────────────────

// syncBlogTags upserts tags by name, then replaces the blog's tag mappings.
func syncBlogTags(tx *gorm.DB, ctx context.Context, blogID string, tagNames []string) error {
	// Delete existing mappings
	if err := tx.WithContext(ctx).Where("blog_id = ?", blogID).Delete(&models.BlogTagMapping{}).Error; err != nil {
		return err
	}

	if len(tagNames) == 0 {
		return nil
	}

	// Upsert each tag
	tagIDs := make([]string, 0, len(tagNames))
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		tagSlug := slugify(name)

		var tag models.BlogTag
		err := tx.WithContext(ctx).Where("slug = ?", tagSlug).First(&tag).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				tag = models.BlogTag{
					Name:      name,
					Slug:      tagSlug,
					CreatedAt: time.Now(),
				}
				if err := tx.WithContext(ctx).Create(&tag).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}
		tagIDs = append(tagIDs, tag.ID)
	}

	// Create new mappings
	for _, tagID := range tagIDs {
		mapping := models.BlogTagMapping{BlogID: blogID, BlogTagID: tagID}
		if err := tx.WithContext(ctx).Create(&mapping).Error; err != nil {
			return err
		}
	}
	return nil
}

// ListBlogTags returns all tags (for autocomplete).
func (s *Store) ListBlogTags(ctx context.Context) ([]models.BlogTag, error) {
	var tags []models.BlogTag
	if err := s.DB.WithContext(ctx).Order("name ASC").Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

// ─── Blog Image operations ─────────────────────────────────────────────────────

// CreateBlogImage inserts a new blog image record.
func (s *Store) CreateBlogImage(ctx context.Context, img *models.BlogImage) error {
	img.CreatedAt = time.Now()
	return s.DB.WithContext(ctx).Create(img).Error
}

// ListBlogImages returns all images for a given blog.
func (s *Store) ListBlogImages(ctx context.Context, blogID string) ([]models.BlogImage, error) {
	var images []models.BlogImage
	if err := s.DB.WithContext(ctx).Where("blog_id = ?", blogID).Order("created_at DESC").Find(&images).Error; err != nil {
		return nil, err
	}
	return images, nil
}

// GetBlogImageByID returns a single blog image by ID.
func (s *Store) GetBlogImageByID(ctx context.Context, imageID string) (*models.BlogImage, error) {
	var img models.BlogImage
	if err := s.DB.WithContext(ctx).Where("id = ?", imageID).First(&img).Error; err != nil {
		return nil, err
	}
	return &img, nil
}

// DeleteBlogImage deletes a blog image record by ID.
func (s *Store) DeleteBlogImage(ctx context.Context, imageID string) error {
	return s.DB.WithContext(ctx).Where("id = ?", imageID).Delete(&models.BlogImage{}).Error
}

// ─── Permission helpers ─────────────────────────────────────────────────────────

// CanEditBlog checks if a user can edit/delete a blog.
// Rules: author themselves, or a higher-role user related to the author
// (student's coach/mentor, coach's mentor, or any admin).
func (s *Store) CanEditBlog(ctx context.Context, requester *models.User, blog *models.Blog) bool {
	// Author can always edit their own blog
	if requester.ID == blog.AuthorID {
		return true
	}
	// Admin can edit any blog
	if requester.Role == models.RoleAdmin {
		return true
	}

	// Load author to check their role
	author, err := s.GetUserByID(ctx, blog.AuthorID)
	if err != nil {
		return false
	}

	switch author.Role {
	case models.RoleStudent:
		// Coach or mentor of this student can edit
		isCoach, _ := s.IsCoachOf(ctx, requester.ID, author.ID)
		if isCoach {
			return true
		}
		isMentor, _ := s.IsMentorOf(ctx, requester.ID, author.ID)
		return isMentor
	case models.RoleCoach:
		// Mentor of this coach can edit
		isMentor, _ := s.IsMentorOf(ctx, requester.ID, author.ID)
		return isMentor
	default:
		return false
	}
}
