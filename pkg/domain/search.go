package domain

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blevesearch/bleve/v2"
)

// Searcher handles full-text search using bleve
type Searcher struct {
	indexPath string
	index     bleve.Index
}

// NewSearcher creates a new Searcher with a bleve index
func NewSearcher(skillsDir string) (*Searcher, error) {
	indexPath := filepath.Join(skillsDir, ".index")

	// Try to open existing index
	index, err := bleve.Open(indexPath)
	if err != nil {
		// Create new index if it doesn't exist
		mapping := bleve.NewIndexMapping()
		index, err = bleve.New(indexPath, mapping)
		if err != nil {
			return nil, fmt.Errorf("failed to create search index: %w", err)
		}
	}

	return &Searcher{
		indexPath: indexPath,
		index:     index,
	}, nil
}

// IndexSkills indexes a list of skills
func (s *Searcher) IndexSkills(skills []Skill) error {
	// Clear existing index by deleting and recreating
	s.index.Close()
	os.RemoveAll(s.indexPath)

	mapping := bleve.NewIndexMapping()
	index, err := bleve.New(s.indexPath, mapping)
	if err != nil {
		return fmt.Errorf("failed to recreate index: %w", err)
	}
	s.index = index

	// Index each skill
	for _, skill := range skills {
		doc := map[string]interface{}{
			"name":    skill.Name,
			"content": skill.Content,
		}
		if skill.Metadata != nil {
			if skill.Metadata.Description != "" {
				doc["description"] = skill.Metadata.Description
			}
			// Index metadata fields if present
			if skill.Metadata.License != "" {
				doc["license"] = skill.Metadata.License
			}
			if skill.Metadata.Compatibility != "" {
				doc["compatibility"] = skill.Metadata.Compatibility
			}
		}
		if err := index.Index(skill.Name, doc); err != nil {
			return fmt.Errorf("failed to index skill %s: %w", skill.Name, err)
		}
	}

	return nil
}

// Search performs a full-text search and returns matching skills
func (s *Searcher) Search(query string) ([]Skill, error) {
	if s.index == nil {
		return []Skill{}, nil
	}

	// Create a disjunction query to search across multiple fields
	contentQuery := bleve.NewMatchQuery(query)
	contentQuery.SetField("content")

	nameQuery := bleve.NewMatchQuery(query)
	nameQuery.SetField("name")

	descQuery := bleve.NewMatchQuery(query)
	descQuery.SetField("description")

	licenseQuery := bleve.NewMatchQuery(query)
	licenseQuery.SetField("license")

	compatibilityQuery := bleve.NewMatchQuery(query)
	compatibilityQuery.SetField("compatibility")

	disjunction := bleve.NewDisjunctionQuery(contentQuery, nameQuery, descQuery, licenseQuery, compatibilityQuery)

	req := bleve.NewSearchRequest(disjunction)
	req.Size = 100 // Limit results

	searchResults, err := s.index.Search(req)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	var skills []Skill
	for _, hit := range searchResults.Hits {
		// The hit.ID is the skill name/ID used for indexing
		skills = append(skills, Skill{
			Name: hit.ID,
			ID:   hit.ID, // ID is the same as Name
		})
	}

	return skills, nil
}

// Close closes the search index
func (s *Searcher) Close() error {
	if s.index != nil {
		return s.index.Close()
	}
	return nil
}
