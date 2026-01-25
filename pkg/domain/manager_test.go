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

		It("should list markdown files as skills", func() {
			// Create a dummy markdown file
			err := os.WriteFile(filepath.Join(tempDir, "docker-guide.md"), []byte("# Docker"), 0644)
			Expect(err).NotTo(HaveOccurred())

			skills, err := manager.ListSkills()
			Expect(err).NotTo(HaveOccurred())
			Expect(skills).To(HaveLen(1))
			Expect(skills[0].Name).To(Equal("docker-guide"))
		})

		It("should ignore non-markdown files", func() {
			err := os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("Not a skill"), 0644)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(tempDir, "skill.md"), []byte("# Skill"), 0644)
			Expect(err).NotTo(HaveOccurred())

			skills, err := manager.ListSkills()
			Expect(err).NotTo(HaveOccurred())
			Expect(skills).To(HaveLen(1))
			Expect(skills[0].Name).To(Equal("skill"))
		})
	})

	Context("Reading Skills", func() {
		It("should read the content of a skill file", func() {
			content := "# Docker Guide\n\nThis is a guide about Docker."
			err := os.WriteFile(filepath.Join(tempDir, "docker.md"), []byte(content), 0644)
			Expect(err).NotTo(HaveOccurred())

			skill, err := manager.ReadSkill("docker")
			Expect(err).NotTo(HaveOccurred())
			Expect(skill.Name).To(Equal("docker"))
			Expect(skill.Content).To(Equal(content))
		})

		It("should return an error for non-existent skill", func() {
			_, err := manager.ReadSkill("nonexistent")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Searching Skills", func() {
		BeforeEach(func() {
			// Create multiple skills for search tests
			err := os.WriteFile(filepath.Join(tempDir, "docker.md"), []byte("# Docker\n\nDocker is a containerization platform."), 0644)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(tempDir, "kubernetes.md"), []byte("# Kubernetes\n\nKubernetes is an orchestration platform."), 0644)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(tempDir, "linux.md"), []byte("# Linux\n\nLinux is an operating system."), 0644)
			Expect(err).NotTo(HaveOccurred())

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
			content := `---
tags: [docker, containers]
description: A guide to Docker
---
# Docker Guide
Content here.`
			err := os.WriteFile(filepath.Join(tempDir, "docker.md"), []byte(content), 0644)
			Expect(err).NotTo(HaveOccurred())

			skill, err := manager.ReadSkill("docker")
			Expect(err).NotTo(HaveOccurred())
			Expect(skill.Metadata).NotTo(BeNil())
			Expect(skill.Metadata.Description).To(Equal("A guide to Docker"))
		})

		It("should handle skills without frontmatter", func() {
			content := "# Docker Guide\nContent here."
			err := os.WriteFile(filepath.Join(tempDir, "docker.md"), []byte(content), 0644)
			Expect(err).NotTo(HaveOccurred())

			skill, err := manager.ReadSkill("docker")
			Expect(err).NotTo(HaveOccurred())
			Expect(skill.Metadata).To(BeNil())
		})
	})
})
