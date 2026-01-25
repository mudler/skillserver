package domain_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/skillserver/pkg/domain"
)

var _ = Describe("Resource Management", func() {
	var (
		manager *domain.FileSystemManager
		tempDir string
		err     error
	)

	BeforeEach(func() {
		// Create a temp directory for each test
		tempDir, err = os.MkdirTemp("", "skillserver-resource-test")
		Expect(err).NotTo(HaveOccurred())

		// Initialize the manager
		manager, err = domain.NewFileSystemManager(tempDir, []string{})
		Expect(err).NotTo(HaveOccurred())

		// Create a test skill with SKILL.md
		skillDir := filepath.Join(tempDir, "test-skill")
		err = os.MkdirAll(skillDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		skillMdContent := `---
name: test-skill
description: A test skill for resource management
---
# Test Skill
Content here.
`
		err = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMdContent), 0644)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Context("Listing Resources", func() {
		It("should return empty list when no resources exist", func() {
			resources, err := manager.ListSkillResources("test-skill")
			Expect(err).NotTo(HaveOccurred())
			Expect(resources).To(BeEmpty())
		})

		It("should list scripts in scripts directory", func() {
			scriptsDir := filepath.Join(tempDir, "test-skill", "scripts")
			err := os.MkdirAll(scriptsDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			scriptContent := "#!/usr/bin/env python3\nprint('Hello')"
			err = os.WriteFile(filepath.Join(scriptsDir, "hello.py"), []byte(scriptContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			resources, err := manager.ListSkillResources("test-skill")
			Expect(err).NotTo(HaveOccurred())
			Expect(resources).To(HaveLen(1))
			Expect(resources[0].Type).To(Equal(domain.ResourceTypeScript))
			Expect(resources[0].Path).To(Equal("scripts/hello.py"))
			Expect(resources[0].Name).To(Equal("hello.py"))
			Expect(resources[0].Readable).To(BeTrue())
		})

		It("should list references in references directory", func() {
			refsDir := filepath.Join(tempDir, "test-skill", "references")
			err := os.MkdirAll(refsDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			refContent := "# Reference\n\nSome reference content."
			err = os.WriteFile(filepath.Join(refsDir, "REFERENCE.md"), []byte(refContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			resources, err := manager.ListSkillResources("test-skill")
			Expect(err).NotTo(HaveOccurred())
			Expect(resources).To(HaveLen(1))
			Expect(resources[0].Type).To(Equal(domain.ResourceTypeReference))
			Expect(resources[0].Path).To(Equal("references/REFERENCE.md"))
		})

		It("should list assets in assets directory", func() {
			assetsDir := filepath.Join(tempDir, "test-skill", "assets")
			err := os.MkdirAll(assetsDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			// Create a binary file (simulated)
			binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
			err = os.WriteFile(filepath.Join(assetsDir, "image.png"), binaryContent, 0644)
			Expect(err).NotTo(HaveOccurred())

			resources, err := manager.ListSkillResources("test-skill")
			Expect(err).NotTo(HaveOccurred())
			Expect(resources).To(HaveLen(1))
			Expect(resources[0].Type).To(Equal(domain.ResourceTypeAsset))
			Expect(resources[0].Path).To(Equal("assets/image.png"))
			Expect(resources[0].Readable).To(BeFalse())
		})

		It("should list resources from all directories", func() {
			// Create resources in all three directories
			scriptsDir := filepath.Join(tempDir, "test-skill", "scripts")
			refsDir := filepath.Join(tempDir, "test-skill", "references")
			assetsDir := filepath.Join(tempDir, "test-skill", "assets")

			os.MkdirAll(scriptsDir, 0755)
			os.MkdirAll(refsDir, 0755)
			os.MkdirAll(assetsDir, 0755)

			os.WriteFile(filepath.Join(scriptsDir, "script.py"), []byte("print('test')"), 0644)
			os.WriteFile(filepath.Join(refsDir, "ref.md"), []byte("# Ref"), 0644)
			os.WriteFile(filepath.Join(assetsDir, "asset.txt"), []byte("asset"), 0644)

			resources, err := manager.ListSkillResources("test-skill")
			Expect(err).NotTo(HaveOccurred())
			Expect(resources).To(HaveLen(3))
		})
	})

	Context("Reading Resources", func() {
		BeforeEach(func() {
			scriptsDir := filepath.Join(tempDir, "test-skill", "scripts")
			os.MkdirAll(scriptsDir, 0755)
			os.WriteFile(filepath.Join(scriptsDir, "hello.py"), []byte("print('Hello, World!')"), 0644)
		})

		It("should read text resource as UTF-8", func() {
			content, err := manager.ReadSkillResource("test-skill", "scripts/hello.py")
			Expect(err).NotTo(HaveOccurred())
			Expect(content.Encoding).To(Equal("utf-8"))
			Expect(content.Content).To(Equal("print('Hello, World!')"))
			Expect(content.MimeType).To(ContainSubstring("python"))
		})

		It("should read binary resource as base64", func() {
			assetsDir := filepath.Join(tempDir, "test-skill", "assets")
			os.MkdirAll(assetsDir, 0755)
			// Create a file with null bytes to ensure it's detected as binary
			binaryData := make([]byte, 100)
			for i := range binaryData {
				if i%10 == 0 {
					binaryData[i] = 0 // Add null bytes
				} else {
					binaryData[i] = byte(i)
				}
			}
			os.WriteFile(filepath.Join(assetsDir, "test.bin"), binaryData, 0644)

			content, err := manager.ReadSkillResource("test-skill", "assets/test.bin")
			Expect(err).NotTo(HaveOccurred())
			Expect(content.Encoding).To(Equal("base64"))
			Expect(content.Content).NotTo(BeEmpty())
		})

		It("should return error for non-existent resource", func() {
			_, err := manager.ReadSkillResource("test-skill", "scripts/nonexistent.py")
			Expect(err).To(HaveOccurred())
		})

		It("should return error for invalid path", func() {
			_, err := manager.ReadSkillResource("test-skill", "../invalid")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Getting Resource Info", func() {
		BeforeEach(func() {
			scriptsDir := filepath.Join(tempDir, "test-skill", "scripts")
			os.MkdirAll(scriptsDir, 0755)
			os.WriteFile(filepath.Join(scriptsDir, "script.py"), []byte("print('test')"), 0644)
		})

		It("should return resource metadata", func() {
			info, err := manager.GetSkillResourceInfo("test-skill", "scripts/script.py")
			Expect(err).NotTo(HaveOccurred())
			Expect(info).NotTo(BeNil())
			Expect(info.Type).To(Equal(domain.ResourceTypeScript))
			Expect(info.Path).To(Equal("scripts/script.py"))
			Expect(info.Name).To(Equal("script.py"))
			Expect(info.Size).To(BeNumerically(">", 0))
			Expect(info.Readable).To(BeTrue())
		})

		It("should return error for non-existent resource", func() {
			_, err := manager.GetSkillResourceInfo("test-skill", "scripts/nonexistent.py")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Path Validation", func() {
		It("should validate resource paths", func() {
			validPaths := []string{
				"scripts/test.py",
				"references/doc.md",
				"assets/image.png",
			}

			for _, path := range validPaths {
				err := domain.ValidateResourcePath(path)
				Expect(err).NotTo(HaveOccurred(), "path %s should be valid", path)
			}
		})

		It("should reject invalid paths", func() {
			invalidPaths := []string{
				"../invalid",
				"/absolute/path",
				"invalid/path",
				"scripts/../../etc/passwd",
			}

			for _, path := range invalidPaths {
				err := domain.ValidateResourcePath(path)
				Expect(err).To(HaveOccurred(), "path %s should be invalid", path)
			}
		})
	})
})
