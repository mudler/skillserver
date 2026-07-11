package domain_test

import (
	"os"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/skillserver/pkg/domain"
)

var _ = Describe("Searcher", func() {
	var (
		searcher *domain.Searcher
		tempDir  string
		err      error
	)

	BeforeEach(func() {
		tempDir, err = os.MkdirTemp("", "skillserver-search-test")
		Expect(err).NotTo(HaveOccurred())
		searcher, err = domain.NewSearcher(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = searcher.Close()
		os.RemoveAll(tempDir)
	})

	Context("Concurrent rebuilds", func() {
		It("does not double-close the index when IndexSkills runs concurrently", func() {
			skills := []domain.Skill{
				{Name: "alpha", Content: "docker guide"},
				{Name: "beta", Content: "kubernetes guide"},
			}

			// Regression: one GitSyncer goroutine per configured repo calls
			// RebuildIndex -> IndexSkills. Without a mutex around index
			// Close/recreate, overlapping calls close an already-closed bleve
			// index and panic with "close of closed channel", crashing the host
			// process. GinkgoRecover surfaces such a panic as a spec failure.
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					Expect(searcher.IndexSkills(skills)).To(Succeed())
				}()
			}
			wg.Wait()

			// Index is still consistent and searchable afterwards.
			results, err := searcher.Search("docker")
			Expect(err).NotTo(HaveOccurred())
			Expect(results).NotTo(BeNil())
		})

		It("stays searchable while rebuilds and searches interleave", func() {
			skills := []domain.Skill{{Name: "gamma", Content: "terraform guide"}}
			Expect(searcher.IndexSkills(skills)).To(Succeed())

			var wg sync.WaitGroup
			for i := 0; i < 6; i++ {
				wg.Add(2)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					Expect(searcher.IndexSkills(skills)).To(Succeed())
				}()
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					_, err := searcher.Search("terraform")
					Expect(err).NotTo(HaveOccurred())
				}()
			}
			wg.Wait()
		})
	})
})
