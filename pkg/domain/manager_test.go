package domain_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/skillserver/pkg/domain"
)

var _ = Describe("SkillManager", func() {
	var (
		manager *domain.FileSystemManager
		tempDir string
		err     error
	)

	BeforeEach(func() {
		// Create a temp directory for each test
		tempDir, err = os.MkdirTemp("", "skillserver-test")
		Expect(err).NotTo(HaveOccurred())

		// Initialize the manager
		manager, err = domain.NewFileSystemManager(tempDir, []string{})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Context("Listing Skills", func() {
		It("should return an empty list when directory is empty", func() {
			skills, err := manager.ListSkills()
			Expect(err).NotTo(HaveOccurred())
			Expect(skills).To(BeEmpty())
		})

		It("should list directories with SKILL.md as skills", func() {
			// Create a skill directory with SKILL.md
			skillDir := filepath.Join(tempDir, "docker-guide")
			err := os.MkdirAll(skillDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			
			skillMdContent := `---
name: docker-guide
description: A guide to Docker
---
# Docker Guide
Content here.
`
			err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMdContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			skills, err := manager.ListSkills()
			Expect(err).NotTo(HaveOccurred())
			Expect(skills).To(HaveLen(1))
			Expect(skills[0].Name).To(Equal("docker-guide"))
		})

		It("should ignore directories without SKILL.md", func() {
			// Create a directory without SKILL.md
			otherDir := filepath.Join(tempDir, "other-dir")
			err := os.MkdirAll(otherDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(otherDir, "readme.txt"), []byte("Not a skill"), 0644)
			Expect(err).NotTo(HaveOccurred())
			
			// Create a valid skill
			skillDir := filepath.Join(tempDir, "valid-skill")
			err = os.MkdirAll(skillDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			skillMdContent := `---
name: valid-skill
description: A valid skill
---
# Valid Skill
Content.
`
			err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMdContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			skills, err := manager.ListSkills()
			Expect(err).NotTo(HaveOccurred())
			Expect(skills).To(HaveLen(1))
			Expect(skills[0].Name).To(Equal("valid-skill"))
		})
	})

	Context("Reading Skills", func() {
		It("should read the content of a skill file", func() {
			skillDir := filepath.Join(tempDir, "docker")
			err := os.MkdirAll(skillDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			
			skillMdContent := `---
name: docker
description: A guide to Docker
---
# Docker Guide

This is a guide about Docker.
`
			err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMdContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			skill, err := manager.ReadSkill("docker")
			Expect(err).NotTo(HaveOccurred())
			Expect(skill.Name).To(Equal("docker"))
			Expect(skill.Content).To(ContainSubstring("Docker Guide"))
		})

		It("should return an error for non-existent skill", func() {
			_, err := manager.ReadSkill("nonexistent")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Searching Skills", func() {
		BeforeEach(func() {
			// Create multiple skills for search tests
			createSkill := func(name, description, content string) {
				skillDir := filepath.Join(tempDir, name)
				os.MkdirAll(skillDir, 0755)
				skillMdContent := `---
name: ` + name + `
description: ` + description + `
---
` + content
				os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMdContent), 0644)
			}
			
			createSkill("docker", "Docker guide", "# Docker\n\nDocker is a containerization platform.")
			createSkill("kubernetes", "Kubernetes guide", "# Kubernetes\n\nKubernetes is an orchestration platform.")
			createSkill("linux", "Linux guide", "# Linux\n\nLinux is an operating system.")

			// Rebuild index after creating files
			err = manager.RebuildIndex()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should find skills by content", func() {
			results, err := manager.SearchSkills("containerization")
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].Name).To(Equal("docker"))
		})

		It("should find skills by title", func() {
			results, err := manager.SearchSkills("Kubernetes")
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].Name).To(Equal("kubernetes"))
		})

		It("should return empty results for non-matching query", func() {
			results, err := manager.SearchSkills("nonexistent")
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(BeEmpty())
		})
	})

	Context("YAML Frontmatter", func() {
		It("should parse YAML frontmatter if present", func() {
			skillDir := filepath.Join(tempDir, "docker")
			err := os.MkdirAll(skillDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			
			content := `---
name: docker
description: A guide to Docker
---
# Docker Guide
Content here.`
			err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)
			Expect(err).NotTo(HaveOccurred())

			skill, err := manager.ReadSkill("docker")
			Expect(err).NotTo(HaveOccurred())
			Expect(skill.Metadata).NotTo(BeNil())
			Expect(skill.Metadata.Description).To(Equal("A guide to Docker"))
		})

		It("should require frontmatter", func() {
			skillDir := filepath.Join(tempDir, "docker")
			err := os.MkdirAll(skillDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			
			content := "# Docker Guide\nContent here."
			err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)
			Expect(err).NotTo(HaveOccurred())

			_, err = manager.ReadSkill("docker")
			Expect(err).To(HaveOccurred()) // Should fail because frontmatter is required
		})
	})

	Context("Git Repository Filtering", func() {
		It("should filter out skills from disabled git repos", func() {
			// Create a git repo structure
			repoDir := filepath.Join(tempDir, "enabled-repo")
			err := os.MkdirAll(repoDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillDir1 := filepath.Join(repoDir, "skill1")
			err = os.MkdirAll(skillDir1, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillMdContent1 := `---
name: skill1
description: Skill from enabled repo
---
# Skill 1
`
			err = os.WriteFile(filepath.Join(skillDir1, "SKILL.md"), []byte(skillMdContent1), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Create disabled repo
			disabledRepoDir := filepath.Join(tempDir, "disabled-repo")
			err = os.MkdirAll(disabledRepoDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillDir2 := filepath.Join(disabledRepoDir, "skill2")
			err = os.MkdirAll(skillDir2, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillMdContent2 := `---
name: skill2
description: Skill from disabled repo
---
# Skill 2
`
			err = os.WriteFile(filepath.Join(skillDir2, "SKILL.md"), []byte(skillMdContent2), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Update manager's git repos list (instead of creating new manager to avoid RebuildIndex)
			manager.UpdateGitRepos([]string{"enabled-repo"})

			// List skills - should only include skill from enabled repo
			skills, err := manager.ListSkills()
			Expect(err).NotTo(HaveOccurred())
			Expect(skills).To(HaveLen(1))
			Expect(skills[0].Name).To(Equal("enabled-repo/skill1"))
		})

		It("should update git repos list dynamically", func() {
			// Create two repos
			repo1Dir := filepath.Join(tempDir, "repo1")
			err := os.MkdirAll(repo1Dir, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillDir1 := filepath.Join(repo1Dir, "skill1")
			err = os.MkdirAll(skillDir1, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillMdContent1 := `---
name: skill1
description: Skill 1
---
# Skill 1
`
			err = os.WriteFile(filepath.Join(skillDir1, "SKILL.md"), []byte(skillMdContent1), 0644)
			Expect(err).NotTo(HaveOccurred())

			repo2Dir := filepath.Join(tempDir, "repo2")
			err = os.MkdirAll(repo2Dir, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillDir2 := filepath.Join(repo2Dir, "skill2")
			err = os.MkdirAll(skillDir2, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillMdContent2 := `---
name: skill2
description: Skill 2
---
# Skill 2
`
			err = os.WriteFile(filepath.Join(skillDir2, "SKILL.md"), []byte(skillMdContent2), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Update manager's git repos list (use existing manager to avoid RebuildIndex)
			manager.UpdateGitRepos([]string{"repo1"})

			skills, err := manager.ListSkills()
			Expect(err).NotTo(HaveOccurred())
			Expect(skills).To(HaveLen(1))
			Expect(skills[0].Name).To(Equal("repo1/skill1"))

			// Update to include both repos
			manager.UpdateGitRepos([]string{"repo1", "repo2"})
			skills, err = manager.ListSkills()
			Expect(err).NotTo(HaveOccurred())
			Expect(skills).To(HaveLen(2))

			// Update to only repo2
			manager.UpdateGitRepos([]string{"repo2"})
			skills, err = manager.ListSkills()
			Expect(err).NotTo(HaveOccurred())
			Expect(skills).To(HaveLen(1))
			Expect(skills[0].Name).To(Equal("repo2/skill2"))
		})
	})
})
