package integration_test

import (
	"encoding/json"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"github.com/onsi/gomega/gexec"
)

var baggageClaimPath string
var fsMounterPath string

func TestIntegration(t *testing.T) {
	rand.Seed(time.Now().Unix())

	RegisterFailHandler(Fail)
	RunSpecs(t, "Baggage Claim Suite")
}

type suiteData struct {
	BaggageClaimPath string
	FSMounterPath    string
}

var _ = SynchronizedBeforeSuite(func() []byte {
	bcPath, err := gexec.Build("github.com/concourse/baggageclaim/cmd/baggageclaim")
	Ω(err).ShouldNot(HaveOccurred())

	fsmPath, err := gexec.Build("github.com/concourse/baggageclaim/cmd/fs_mounter")
	Ω(err).ShouldNot(HaveOccurred())

	data, err := json.Marshal(suiteData{
		BaggageClaimPath: bcPath,
		FSMounterPath:    fsmPath,
	})
	Ω(err).ShouldNot(HaveOccurred())

	return data
}, func(data []byte) {
	var suiteData suiteData
	err := json.Unmarshal(data, &suiteData)
	Ω(err).ShouldNot(HaveOccurred())

	baggageClaimPath = suiteData.BaggageClaimPath
	fsMounterPath = suiteData.FSMounterPath
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

func NewRunner(path string, port int, volumeDir string) *BaggageClaimRunner {
	return &BaggageClaimRunner{
		path:      path,
		port:      port,
		volumeDir: volumeDir,
	}
}

func (bcr *BaggageClaimRunner) start() {
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

func (bcr *BaggageClaimRunner) stop() {
	bcr.process.Signal(os.Kill)
	Eventually(bcr.process.Wait()).Should(Receive())
}

func (bcr *BaggageClaimRunner) bounce() {
	bcr.stop()
	bcr.start()
}

func (bcr *BaggageClaimRunner) cleanup() {
	err := os.RemoveAll(bcr.volumeDir)
	Ω(err).ShouldNot(HaveOccurred())
}
