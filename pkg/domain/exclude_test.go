package domain_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/skillserver/pkg/domain"
)

func writeSkill(dir, name, body string) {
	skillDir := filepath.Join(dir, name)
	Expect(os.MkdirAll(filepath.Join(skillDir, "references"), 0755)).To(Succeed())
	md := fmt.Sprintf("---\nname: %s\ndescription: %s skill\n---\n%s\n", name, name, body)
	Expect(os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(skillDir, "references", "notes.md"), []byte("notes"), 0644)).To(Succeed())
}

func skillNames(skills []domain.Skill) []string {
	names := make([]string, 0, len(skills))
	for _, s := range skills {
		names = append(names, s.Name)
	}
	return names
}

var _ = Describe("Excluding skills by name", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "skillserver-exclude")
		Expect(err).NotTo(HaveOccurred())

		// A git-synced repo "superpowers" with two nested skills and a local skill.
		repoSkills := filepath.Join(tempDir, "superpowers", "skills")
		Expect(os.MkdirAll(filepath.Join(tempDir, "superpowers", ".git"), 0755)).To(Succeed())
		writeSkill(repoSkills, "using-superpowers", "Invoke a skill before every answer. zebracorn")
		writeSkill(repoSkills, "brainstorming", "Explore intent before building. zebracorn")
		writeSkill(tempDir, "local-helper", "A local helper. zebracorn")
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
		os.Unsetenv(domain.ExcludeSkillsEnv)
	})

	newManager := func(opts ...domain.Option) *domain.FileSystemManager {
		m, err := domain.NewFileSystemManager(tempDir, []string{"superpowers"}, opts...)
		Expect(err).NotTo(HaveOccurred())
		return m
	}

	It("lists, searches and reads every skill when no exclusion is configured", func() {
		m := newManager()
		skills, err := m.ListSkills()
		Expect(err).NotTo(HaveOccurred())
		Expect(skillNames(skills)).To(ConsistOf("superpowers/using-superpowers", "superpowers/brainstorming", "local-helper"))

		found, err := m.SearchSkills("zebracorn")
		Expect(err).NotTo(HaveOccurred())
		Expect(skillNames(found)).To(ContainElement("superpowers/using-superpowers"))

		_, err = m.ReadSkill("superpowers/using-superpowers")
		Expect(err).NotTo(HaveOccurred())
	})

	It("hides an excluded skill from list, search, read and resources but keeps the rest of the repo", func() {
		m := newManager(domain.WithExcludedSkills("superpowers/using-superpowers"))

		skills, err := m.ListSkills()
		Expect(err).NotTo(HaveOccurred())
		Expect(skillNames(skills)).To(ConsistOf("superpowers/brainstorming", "local-helper"))

		found, err := m.SearchSkills("zebracorn")
		Expect(err).NotTo(HaveOccurred())
		Expect(skillNames(found)).NotTo(ContainElement("superpowers/using-superpowers"))
		Expect(skillNames(found)).To(ContainElement("superpowers/brainstorming"))

		_, err = m.ReadSkill("superpowers/using-superpowers")
		Expect(err).To(MatchError(ContainSubstring("skill not found")))
		_, err = m.ListSkillResources("superpowers/using-superpowers")
		Expect(err).To(HaveOccurred())
		_, err = m.ReadSkillResource("superpowers/using-superpowers", "references/notes.md")
		Expect(err).To(HaveOccurred())

		// Counter-check: sibling skill of the same repo stays readable.
		_, err = m.ReadSkill("superpowers/brainstorming")
		Expect(err).NotTo(HaveOccurred())
		_, err = m.ReadSkillResource("superpowers/brainstorming", "references/notes.md")
		Expect(err).NotTo(HaveOccurred())

		// Files on disk are untouched (git pull must keep working).
		_, err = os.Stat(filepath.Join(tempDir, "superpowers", "skills", "using-superpowers", "SKILL.md"))
		Expect(err).NotTo(HaveOccurred())
	})

	It("supports glob patterns", func() {
		m := newManager(domain.WithExcludedSkills("*/using-*", "local-*"))
		skills, err := m.ListSkills()
		Expect(err).NotTo(HaveOccurred())
		Expect(skillNames(skills)).To(ConsistOf("superpowers/brainstorming"))
	})

	It("does not let * cross the repo separator", func() {
		m := newManager(domain.WithExcludedSkills("*"))
		skills, err := m.ListSkills()
		Expect(err).NotTo(HaveOccurred())
		Expect(skillNames(skills)).To(ConsistOf("superpowers/using-superpowers", "superpowers/brainstorming"))
	})

	It("reads the patterns from SKILLSERVER_EXCLUDE_SKILLS when no option is given", func() {
		Expect(os.Setenv(domain.ExcludeSkillsEnv, " superpowers/using-superpowers , ")).To(Succeed())
		m := newManager()
		skills, err := m.ListSkills()
		Expect(err).NotTo(HaveOccurred())
		Expect(skillNames(skills)).To(ConsistOf("superpowers/brainstorming", "local-helper"))
	})

	It("lets an explicit option override the environment variable", func() {
		Expect(os.Setenv(domain.ExcludeSkillsEnv, "superpowers/using-superpowers")).To(Succeed())
		m := newManager(domain.WithExcludedSkills())
		skills, err := m.ListSkills()
		Expect(err).NotTo(HaveOccurred())
		Expect(skills).To(HaveLen(3))
	})

	It("keeps the exclusion after the git repo list is updated and the index rebuilt", func() {
		m := newManager(domain.WithExcludedSkills("superpowers/using-superpowers"))
		m.UpdateGitRepos([]string{"superpowers"})
		Expect(m.RebuildIndex()).To(Succeed())
		found, err := m.SearchSkills("zebracorn")
		Expect(err).NotTo(HaveOccurred())
		Expect(skillNames(found)).NotTo(ContainElement("superpowers/using-superpowers"))
	})

	It("rejects invalid patterns", func() {
		_, err := domain.NewFileSystemManager(tempDir, nil, domain.WithExcludedSkills("["))
		Expect(err).To(MatchError(ContainSubstring("invalid skill exclude pattern")))
	})
})
