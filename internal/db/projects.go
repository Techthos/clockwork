package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/techthos/clockwork/internal/models"
	"github.com/techthos/clockwork/internal/source"
	bolt "go.etcd.io/bbolt"
)

// ProjectInput carries the fields needed to create a project. SourceType may
// be left empty, in which case it is inferred from whichever locator is set.
type ProjectInput struct {
	Name        string
	SourceType  string
	GitRepoPath string
	Repository  string
}

// ProjectUpdate carries partial project changes. A nil field is left
// untouched; a non-nil field is written, including when it is empty, which is
// how a locator gets cleared.
type ProjectUpdate struct {
	Name        *string
	SourceType  *string
	GitRepoPath *string
	Repository  *string
}

// normalizeProject fills in the effective source type for rows written before
// source_type existed, so callers always observe a concrete value.
func normalizeProject(p *models.Project) *models.Project {
	p.SourceType = source.Resolve(p).String()
	return p
}

// CreateProject creates a new project
func (s *Store) CreateProject(in ProjectInput) (*models.Project, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("project name must not be empty")
	}

	var srcType source.Type
	if raw := strings.TrimSpace(in.SourceType); raw != "" {
		parsed, err := source.Parse(raw)
		if err != nil {
			return nil, err
		}
		srcType = parsed
	} else {
		srcType = source.Infer(in.GitRepoPath, in.Repository)
	}

	if err := source.Validate(srcType, in.GitRepoPath, in.Repository); err != nil {
		return nil, err
	}

	project := &models.Project{
		ID:          uuid.New().String(),
		Name:        name,
		SourceType:  srcType.String(),
		GitRepoPath: in.GitRepoPath,
		Repository:  in.Repository,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := s.update(func(tx *bolt.Tx) error {
		b, err := bucket(tx, projectsBucket)
		if err != nil {
			return err
		}
		data, err := json.Marshal(project)
		if err != nil {
			return err
		}
		return b.Put([]byte(project.ID), data)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return project, nil
}

// GetProject retrieves a project by ID
func (s *Store) GetProject(id string) (*models.Project, error) {
	var project models.Project

	err := s.view(func(tx *bolt.Tx) error {
		b, err := bucket(tx, projectsBucket)
		if err != nil {
			return err
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project not found")
		}
		return json.Unmarshal(data, &project)
	})

	if err != nil {
		return nil, err
	}

	return normalizeProject(&project), nil
}

// UpdateProject applies a partial update to an existing project. Switching
// source type clears the locator that no longer applies, so a project can move
// between lookup methods without leaving stale fields behind.
func (s *Store) UpdateProject(id string, upd ProjectUpdate) (*models.Project, error) {
	var project models.Project

	err := s.update(func(tx *bolt.Tx) error {
		b, err := bucket(tx, projectsBucket)
		if err != nil {
			return err
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("project not found")
		}

		if err := json.Unmarshal(data, &project); err != nil {
			return err
		}
		normalizeProject(&project)

		if upd.Name != nil {
			if strings.TrimSpace(*upd.Name) == "" {
				return fmt.Errorf("project name must not be empty")
			}
			project.Name = strings.TrimSpace(*upd.Name)
		}

		srcType := source.Resolve(&project)
		if upd.SourceType != nil {
			parsed, err := source.Parse(*upd.SourceType)
			if err != nil {
				return err
			}
			srcType = parsed
		}

		// Reject locators that contradict the resolved type outright; silently
		// drop leftovers from the previous type further down.
		if upd.GitRepoPath != nil && *upd.GitRepoPath != "" && srcType != source.Local {
			return fmt.Errorf("git_repo_path only applies to source type %q, not %q", source.Local, srcType)
		}
		if upd.Repository != nil && *upd.Repository != "" && srcType != source.MCP {
			return fmt.Errorf("repository only applies to source type %q, not %q", source.MCP, srcType)
		}
		if upd.GitRepoPath != nil {
			project.GitRepoPath = *upd.GitRepoPath
		}
		if upd.Repository != nil {
			project.Repository = *upd.Repository
		}

		switch srcType {
		case source.None:
			project.GitRepoPath, project.Repository = "", ""
		case source.Local:
			project.Repository = ""
		case source.MCP:
			project.GitRepoPath = ""
		}

		if err := source.Validate(srcType, project.GitRepoPath, project.Repository); err != nil {
			return err
		}
		project.SourceType = srcType.String()
		project.UpdatedAt = time.Now()

		updatedData, err := json.Marshal(project)
		if err != nil {
			return err
		}

		return b.Put([]byte(id), updatedData)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	return &project, nil
}

// DeleteProject deletes a project and all its entries (single-pass cascade).
func (s *Store) DeleteProject(id string) error {
	return s.update(func(tx *bolt.Tx) error {
		pb, err := bucket(tx, projectsBucket)
		if err != nil {
			return err
		}
		if pb.Get([]byte(id)) == nil {
			return fmt.Errorf("project not found")
		}
		if err := pb.Delete([]byte(id)); err != nil {
			return err
		}

		eb, err := bucket(tx, entriesBucket)
		if err != nil {
			return err
		}

		// Collect matching keys first; deleting during cursor iteration is unsafe.
		var toDelete [][]byte
		if err := eb.ForEach(func(k, v []byte) error {
			if v == nil {
				return nil // nested bucket, not an entry
			}
			var entry models.Entry
			if err := json.Unmarshal(v, &entry); err != nil {
				return nil // skip corrupt rows
			}
			if entry.ProjectID == id {
				toDelete = append(toDelete, append([]byte(nil), k...))
			}
			return nil
		}); err != nil {
			return err
		}

		for _, k := range toDelete {
			if err := eb.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListProjects returns all projects
func (s *Store) ListProjects() ([]*models.Project, error) {
	var projects []*models.Project

	err := s.view(func(tx *bolt.Tx) error {
		b, err := bucket(tx, projectsBucket)
		if err != nil {
			return err
		}
		return b.ForEach(func(k, v []byte) error {
			if v == nil {
				return nil // nested bucket, not a project
			}
			var project models.Project
			if err := json.Unmarshal(v, &project); err != nil {
				return fmt.Errorf("decode project %q: %w", k, err)
			}
			projects = append(projects, normalizeProject(&project))
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return projects, nil
}

// projectNames maps project id to name inside an open transaction, so an
// entry search can match the project an entry belongs to even though the entry
// stores only a reference.
func projectNames(tx *bolt.Tx) (map[string]string, error) {
	b, err := bucket(tx, projectsBucket)
	if err != nil {
		return nil, err
	}

	names := make(map[string]string)
	err = b.ForEach(func(k, v []byte) error {
		if v == nil {
			return nil
		}
		var project models.Project
		if err := json.Unmarshal(v, &project); err != nil {
			return nil // a corrupt project must not break an entry search
		}
		names[project.ID] = project.Name
		return nil
	})
	if err != nil {
		return nil, err
	}
	return names, nil
}
