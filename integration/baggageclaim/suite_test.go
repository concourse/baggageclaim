package integration_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/concourse/baggageclaim/integration/baggageclaim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"github.com/onsi/gomega/gexec"
)

var baggageClaimPath string

func TestIntegration(t *testing.T) {
	rand.Seed(time.Now().Unix())

	RegisterFailHandler(Fail)
	RunSpecs(t, "Baggage Claim Suite")
}

type suiteData struct {
	BaggageClaimPath string
}

var _ = SynchronizedBeforeSuite(func() []byte {
	bcPath, err := gexec.Build("github.com/concourse/baggageclaim/cmd/baggageclaim")
	Ω(err).ShouldNot(HaveOccurred())

	data, err := json.Marshal(suiteData{
		BaggageClaimPath: bcPath,
	})
	Ω(err).ShouldNot(HaveOccurred())

	return data
}, func(data []byte) {
	var suiteData suiteData
	err := json.Unmarshal(data, &suiteData)
	Ω(err).ShouldNot(HaveOccurred())

	baggageClaimPath = suiteData.BaggageClaimPath

	// poll less frequently
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	SetDefaultConsistentlyPollingInterval(100 * time.Millisecond)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

type BaggageClaimRunner struct {
	path      string
	process   ifrit.Process
	port      int
	volumeDir string
}

func NewRunner(path string) *BaggageClaimRunner {
	port := 7788 + GinkgoParallelNode()
	volumeDir, err := ioutil.TempDir("", fmt.Sprintf("baggageclaim_volume_dir_%d", GinkgoParallelNode()))
	Ω(err).ShouldNot(HaveOccurred())

	return &BaggageClaimRunner{
		path:      path,
		port:      port,
		volumeDir: volumeDir,
	}
}

func (bcr *BaggageClaimRunner) Start() {
	runner := ginkgomon.New(ginkgomon.Config{
		Name: "baggageclaim",
		Command: exec.Command(
			bcr.path,
			"-listenPort", strconv.Itoa(bcr.port),
			"-volumeDir", bcr.volumeDir,
		),
		StartCheck: "baggageclaim.listening",
	})

	bcr.process = ginkgomon.Invoke(runner)
}

func (bcr *BaggageClaimRunner) Stop() {
	bcr.process.Signal(os.Kill)
	Eventually(bcr.process.Wait()).Should(Receive())
}

func (bcr *BaggageClaimRunner) Bounce() {
	bcr.Stop()
	bcr.Start()
}

func (bcr *BaggageClaimRunner) Cleanup() {
	err := os.RemoveAll(bcr.volumeDir)
	Ω(err).ShouldNot(HaveOccurred())
}

func (bcr *BaggageClaimRunner) Client() *integration.Client {
	return integration.NewClient(fmt.Sprintf("http://localhost:%d", bcr.port))
}

func (bcr *BaggageClaimRunner) VolumeDir() string {
	return bcr.volumeDir
}

func (bcr *BaggageClaimRunner) Port() int {
	return bcr.port
}
