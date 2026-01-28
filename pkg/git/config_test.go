package git_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/skillserver/pkg/git"
)

var _ = Describe("ConfigManager", func() {
	var (
		configManager *git.ConfigManager
		tempDir       string
		err           error
	)

	BeforeEach(func() {
		tempDir, err = os.MkdirTemp("", "skillserver-config-test")
		Expect(err).NotTo(HaveOccurred())

		configManager = git.NewConfigManager(tempDir)
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Context("LoadConfig", func() {
		It("should return empty list when config file doesn't exist", func() {
			repos, err := configManager.LoadConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(BeEmpty())
		})

		It("should load repos from config file", func() {
			// Create config file
			configPath := filepath.Join(tempDir, ".git-repos.json")
			configContent := `[
  {
    "id": "test-repo",
    "url": "https://github.com/user/repo.git",
    "name": "repo",
    "enabled": true
  }
]`
			err := os.WriteFile(configPath, []byte(configContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			repos, err := configManager.LoadConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveLen(1))
			Expect(repos[0].ID).To(Equal("test-repo"))
			Expect(repos[0].URL).To(Equal("https://github.com/user/repo.git"))
			Expect(repos[0].Name).To(Equal("repo"))
			Expect(repos[0].Enabled).To(BeTrue())
		})

		It("should handle disabled repos", func() {
			configPath := filepath.Join(tempDir, ".git-repos.json")
			configContent := `[
  {
    "id": "enabled-repo",
    "url": "https://github.com/user/enabled.git",
    "name": "enabled",
    "enabled": true
  },
  {
    "id": "disabled-repo",
    "url": "https://github.com/user/disabled.git",
    "name": "disabled",
    "enabled": false
  }
]`
			err := os.WriteFile(configPath, []byte(configContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			repos, err := configManager.LoadConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(repos).To(HaveLen(2))
			Expect(repos[0].Enabled).To(BeTrue())
			Expect(repos[1].Enabled).To(BeFalse())
		})
	})

	Context("SaveConfig", func() {
		It("should save repos to config file", func() {
			repos := []git.GitRepoConfig{
				{
					ID:      "repo1",
					URL:     "https://github.com/user/repo1.git",
					Name:    "repo1",
					Enabled: true,
				},
				{
					ID:      "repo2",
					URL:     "https://github.com/user/repo2.git",
					Name:    "repo2",
					Enabled: false,
				},
			}

			err := configManager.SaveConfig(repos)
			Expect(err).NotTo(HaveOccurred())

			// Verify file was created
			configPath := filepath.Join(tempDir, ".git-repos.json")
			Expect(configPath).To(BeAnExistingFile())

			// Load and verify
			loadedRepos, err := configManager.LoadConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(loadedRepos).To(HaveLen(2))
			Expect(loadedRepos[0].ID).To(Equal("repo1"))
			Expect(loadedRepos[1].ID).To(Equal("repo2"))
			Expect(loadedRepos[0].Enabled).To(BeTrue())
			Expect(loadedRepos[1].Enabled).To(BeFalse())
		})

		It("should create directory if it doesn't exist", func() {
			nestedDir := filepath.Join(tempDir, "nested", "path")
			configManager = git.NewConfigManager(nestedDir)

			repos := []git.GitRepoConfig{
				{
					ID:      "repo1",
					URL:     "https://github.com/user/repo1.git",
					Name:    "repo1",
					Enabled: true,
				},
			}

			err := configManager.SaveConfig(repos)
			Expect(err).NotTo(HaveOccurred())

			configPath := filepath.Join(nestedDir, ".git-repos.json")
			Expect(configPath).To(BeAnExistingFile())
		})
	})

	Context("ExtractRepoName", func() {
		It("should extract repo name from HTTPS URL", func() {
			name := git.ExtractRepoName("https://github.com/user/repo.git")
			Expect(name).To(Equal("repo"))
		})

		It("should extract repo name from URL without .git suffix", func() {
			name := git.ExtractRepoName("https://github.com/user/repo")
			Expect(name).To(Equal("repo"))
		})

		It("should extract repo name from SSH URL", func() {
			name := git.ExtractRepoName("git@github.com:user/repo.git")
			Expect(name).To(Equal("repo"))
		})

		It("should handle nested paths", func() {
			name := git.ExtractRepoName("https://github.com/org/group/repo.git")
			Expect(name).To(Equal("repo"))
		})
	})

	Context("GenerateID", func() {
		It("should generate consistent IDs", func() {
			id1 := git.GenerateID("https://github.com/user/repo.git")
			id2 := git.GenerateID("https://github.com/user/repo.git")
			Expect(id1).To(Equal(id2))
		})

		It("should generate different IDs for different repos", func() {
			id1 := git.GenerateID("https://github.com/user/repo1.git")
			id2 := git.GenerateID("https://github.com/user/repo2.git")
			Expect(id1).NotTo(Equal(id2))
		})
	})
})
