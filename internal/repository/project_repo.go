package repository

import (
	"context"
	"errors"
	"time"

	"task-api/internal/domain/project"

	"gorm.io/gorm"
)

// projectModel is the persistence model of the Project aggregate, kept
// separate from the domain entity so the domain stays free of GORM concerns.
type projectModel struct {
	ID        string `gorm:"primaryKey;type:uuid"`
	Name      string `gorm:"type:varchar(255);not null"`
	OwnerID   string `gorm:"type:uuid;not null;column:owner_id"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (projectModel) TableName() string { return "projects" }

// projectMemberModel is the persistence model of the Member child entity.
type projectMemberModel struct {
	ProjectID string    `gorm:"primaryKey;type:uuid;column:project_id"`
	UserID    string    `gorm:"primaryKey;type:uuid;column:user_id"`
	JoinedAt  time.Time `gorm:"column:joined_at"`
}

func (projectMemberModel) TableName() string { return "project_members" }

func projectToDomain(m projectModel) *project.Project {
	return &project.Project{
		ID:        m.ID,
		Name:      m.Name,
		OwnerID:   m.OwnerID,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func projectFromDomain(p *project.Project) projectModel {
	return projectModel{
		ID:        p.ID,
		Name:      p.Name,
		OwnerID:   p.OwnerID,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func memberToDomain(m projectMemberModel) project.Member {
	return project.Member{
		ProjectID: m.ProjectID,
		UserID:    m.UserID,
		JoinedAt:  m.JoinedAt,
	}
}

type projectRepo struct {
	db *gorm.DB
}

// NewProjectRepository returns the concrete type (like NewUserRepository)
// so *projectRepo can also satisfy consumer-side ports, e.g. the task
// application service's MembershipChecker.
func NewProjectRepository(db *gorm.DB) *projectRepo {
	return &projectRepo{db: db}
}

// Create persists the project and the owner's membership atomically: the
// "owner is a member" invariant must commit as a whole or not at all.
func (r *projectRepo) Create(ctx context.Context, p *project.Project) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		m := projectFromDomain(p)
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		owner := projectMemberModel{
			ProjectID: p.ID,
			UserID:    p.OwnerID,
			JoinedAt:  p.CreatedAt,
		}
		return tx.Create(&owner).Error
	})
}

func (r *projectRepo) FindByID(ctx context.Context, id string) (*project.Project, error) {
	var m projectModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Translate the infrastructure error into a domain error so
		// upper layers never depend on GORM.
		return nil, project.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return projectToDomain(m), nil
}

func (r *projectRepo) FindAllByMember(ctx context.Context, userID string) ([]project.Project, error) {
	var models []projectModel
	err := r.db.WithContext(ctx).
		Joins("JOIN project_members pm ON pm.project_id = projects.id").
		Where("pm.user_id = ?", userID).
		Order("projects.created_at DESC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	projects := make([]project.Project, 0, len(models))
	for _, m := range models {
		projects = append(projects, *projectToDomain(m))
	}
	return projects, nil
}

// IsMember reports whether userID belongs to projectID. It backs both the
// repository port and the narrow MembershipChecker port consumed by the
// task application service.
func (r *projectRepo) IsMember(ctx context.Context, projectID, userID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&projectMemberModel{}).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *projectRepo) AddMember(ctx context.Context, m *project.Member) error {
	model := projectMemberModel{
		ProjectID: m.ProjectID,
		UserID:    m.UserID,
		JoinedAt:  m.JoinedAt,
	}
	err := r.db.WithContext(ctx).Create(&model).Error
	// Requires gorm.Config{TranslateError: true}; translates the composite
	// primary key violation into a domain error.
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return project.ErrAlreadyMember
	}
	return err
}

// RemoveMember is idempotent: deleting a non-existing membership affects
// zero rows and is still a success.
func (r *projectRepo) RemoveMember(ctx context.Context, projectID, userID string) error {
	return r.db.WithContext(ctx).
		Delete(&projectMemberModel{}, "project_id = ? AND user_id = ?", projectID, userID).Error
}

func (r *projectRepo) ListMembers(ctx context.Context, projectID string) ([]project.Member, error) {
	var models []projectMemberModel
	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("joined_at ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	members := make([]project.Member, 0, len(models))
	for _, m := range models {
		members = append(members, memberToDomain(m))
	}
	return members, nil
}
