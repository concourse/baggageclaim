package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"github.com/onsi/gomega/gexec"
)

var matterMasterPath string

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Matter Master Suite")
}

type suiteData struct {
	MatterMasterPath string
}

var _ = SynchronizedBeforeSuite(func() []byte {
	mmPath, err := gexec.Build("github.com/concourse/mattermaster/cmd/mattermaster")
	Ω(err).ShouldNot(HaveOccurred())

	data, err := json.Marshal(suiteData{
		MatterMasterPath: mmPath,
	})
	Ω(err).ShouldNot(HaveOccurred())

	return data
}, func(data []byte) {
	var suiteData suiteData
	err := json.Unmarshal(data, &suiteData)
	Ω(err).ShouldNot(HaveOccurred())

	matterMasterPath = suiteData.MatterMasterPath
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

type matterMasterRunner struct {
	path      string
	process   ifrit.Process
	port      int
	volumeDir string
}

func newRunner(path string, port int, volumeDir string) *matterMasterRunner {
	return &matterMasterRunner{
		path:      path,
		port:      port,
		volumeDir: volumeDir,
	}
}

func (mmr *matterMasterRunner) start() {
	runner := ginkgomon.New(ginkgomon.Config{
		Name: "mattermaster",
		Command: exec.Command(
			mmr.path,
			"-listenPort", strconv.Itoa(mmr.port),
			"-volumeDir", mmr.volumeDir,
		),
		StartCheck: "mattermaster.listening",
	})

	mmr.process = ginkgomon.Invoke(runner)
}

func (mmr *matterMasterRunner) stop() {
	mmr.process.Signal(os.Kill)
	Eventually(mmr.process.Wait()).Should(Receive())
}

func (mmr *matterMasterRunner) bounce() {
	mmr.stop()
	mmr.start()
}

func (mmr *matterMasterRunner) cleanup() {
	err := os.RemoveAll(mmr.volumeDir)
	Ω(err).ShouldNot(HaveOccurred())
}
