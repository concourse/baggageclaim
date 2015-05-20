package integration_test

import (
	"os/exec"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Startup", func() {
	var (
		process *gexec.Session
	)

	It("exits with an error if volumeDir is not specified", func() {
		port := 7788 + GinkgoParallelNode()

		command := exec.Command(
			matterMasterPath,
			"-listenPort", strconv.Itoa(port),
		)

		var err error
		process, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(process.Err).Should(gbytes.Say("-volumeDir must be specified"))
		Eventually(process).Should(gexec.Exit(1))
	})

	AfterEach(func() {
		process.Kill().Wait(1 * time.Second)
	})
})
