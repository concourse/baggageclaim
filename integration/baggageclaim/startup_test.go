package integration_test

import (
	"io/ioutil"
	"os/exec"
	"time"

	"github.com/concourse/baggageclaim/baggageclaimcmd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v2"
)

var _ = Describe("Startup", func() {
	var (
		process *gexec.Session
	)

	AfterEach(func() {
		process.Kill().Wait(1 * time.Second)
	})

	It("exits with an error if volumes is not specified", func() {
		port := 7788 + GinkgoParallelNode()

		config := baggageclaimcmd.BaggageclaimConfig{
			BindPort: uint16(port),
		}

		configYAML, err := yaml.Marshal(config)
		Expect(err).ToNot(HaveOccurred())

		configFile, err := ioutil.TempFile("", "config.yml")
		Expect(err).NotTo(HaveOccurred())

		defer configFile.Close()

		_, err = configFile.Write(configYAML)
		Expect(err).NotTo(HaveOccurred())

		command := exec.Command(
			baggageClaimPath,
			"--config", configFile.Name(),
		)

		process, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(process.Err).Should(gbytes.Say("VolumesDir is a required field"))
		Eventually(process).Should(gexec.Exit(1))
	})
})
