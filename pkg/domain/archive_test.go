package domain_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/skillserver/pkg/domain"
)

var _ = Describe("Archive", func() {
	var (
		tempDir string
		err     error
	)

	BeforeEach(func() {
		tempDir, err = os.MkdirTemp("", "skillserver-archive-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Context("ExportSkill", func() {
		It("should create a valid tar.gz archive", func() {
			// Create a skill directory
			skillDir := filepath.Join(tempDir, "test-skill")
			err := os.MkdirAll(skillDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			// Create SKILL.md
			skillMdContent := `---
name: test-skill
description: A test skill
---
# Test Skill
Content here.
`
			err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMdContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Create a resource file
			scriptsDir := filepath.Join(skillDir, "scripts")
			err = os.MkdirAll(scriptsDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(scriptsDir, "script.sh"), []byte("#!/bin/bash\necho hello"), 0755)
			Expect(err).NotTo(HaveOccurred())

			// Export the skill
			archiveData, err := domain.ExportSkill("test-skill", tempDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(archiveData).NotTo(BeEmpty())
			Expect(len(archiveData)).To(BeNumerically(">", 100)) // Should be a reasonable size
		})

		It("should export git repo skills", func() {
			// Create a git repo structure
			repoDir := filepath.Join(tempDir, "test-repo")
			err := os.MkdirAll(repoDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillDir := filepath.Join(repoDir, "nested-skill")
			err = os.MkdirAll(skillDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillMdContent := `---
name: nested-skill
description: A nested skill
---
# Nested Skill
Content.
`
			err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMdContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Export the skill
			archiveData, err := domain.ExportSkill("test-repo/nested-skill", tempDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(archiveData).NotTo(BeEmpty())
		})

		It("should return an error for non-existent skill", func() {
			_, err := domain.ExportSkill("nonexistent", tempDir)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("ImportSkill", func() {
		It("should import a valid archive", func() {
			// Create a skill directory (name must match frontmatter)
			skillDir := filepath.Join(tempDir, "imported-skill")
			err := os.MkdirAll(skillDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillMdContent := `---
name: imported-skill
description: An imported skill
---
# Imported Skill
Content here.
`
			err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMdContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Create a resource
			scriptsDir := filepath.Join(skillDir, "scripts")
			err = os.MkdirAll(scriptsDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(scriptsDir, "test.sh"), []byte("#!/bin/bash"), 0755)
			Expect(err).NotTo(HaveOccurred())

			// Export first
			archiveData, err := domain.ExportSkill("imported-skill", tempDir)
			Expect(err).NotTo(HaveOccurred())

			// Remove source skill
			os.RemoveAll(skillDir)

			// Import to a different location
			importDir := filepath.Join(tempDir, "imported")
			err = os.MkdirAll(importDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillName, err := domain.ImportSkill(archiveData, importDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(skillName).To(Equal("imported-skill"))

			// Verify skill was imported
			importedSkillDir := filepath.Join(importDir, "imported-skill")
			Expect(importedSkillDir).To(BeADirectory())
			Expect(filepath.Join(importedSkillDir, "SKILL.md")).To(BeAnExistingFile())
			Expect(filepath.Join(importedSkillDir, "scripts", "test.sh")).To(BeAnExistingFile())
		})

		It("should validate skill structure on import", func() {
			// Create invalid archive (missing SKILL.md)
			skillDir := filepath.Join(tempDir, "invalid-skill")
			err := os.MkdirAll(skillDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(skillDir, "readme.txt"), []byte("Not a skill"), 0644)
			Expect(err).NotTo(HaveOccurred())

			// Try to export (will fail)
			_, err = domain.ExportSkill("invalid-skill", tempDir)
			Expect(err).To(HaveOccurred())
		})

		It("should reject archive with invalid skill name", func() {
			// Create archive with invalid name
			skillDir := filepath.Join(tempDir, "INVALID-NAME")
			err := os.MkdirAll(skillDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillMdContent := `---
name: INVALID-NAME
description: Invalid name
---
# Invalid
`
			err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMdContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			archiveData, err := domain.ExportSkill("INVALID-NAME", tempDir)
			Expect(err).NotTo(HaveOccurred())

			// Import should fail validation
			importDir := filepath.Join(tempDir, "imported")
			os.MkdirAll(importDir, 0755)
			_, err = domain.ImportSkill(archiveData, importDir)
			Expect(err).To(HaveOccurred())
		})

		It("should reject duplicate skill on import", func() {
			// Create and export a skill
			skillDir := filepath.Join(tempDir, "duplicate-skill")
			err := os.MkdirAll(skillDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			skillMdContent := `---
name: duplicate-skill
description: A duplicate skill
---
# Duplicate
`
			err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMdContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			archiveData, err := domain.ExportSkill("duplicate-skill", tempDir)
			Expect(err).NotTo(HaveOccurred())

			// Try to import again (should fail)
			_, err = domain.ImportSkill(archiveData, tempDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("already exists"))
		})
	})
})
