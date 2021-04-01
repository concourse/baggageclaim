package integration_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/baggageclaimcmd"
	"github.com/concourse/baggageclaim/client"
	"github.com/concourse/flag"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"gopkg.in/yaml.v2"

	"github.com/onsi/gomega/gexec"
)

var logger lager.Logger
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
	Expect(err).NotTo(HaveOccurred())

	data, err := json.Marshal(suiteData{
		BaggageClaimPath: bcPath,
	})
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	var suiteData suiteData
	err := json.Unmarshal(data, &suiteData)
	Expect(err).NotTo(HaveOccurred())

	logger = lagertest.NewTestLogger("test")
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
	driver    string
}

func NewRunner(path string, driver string) *BaggageClaimRunner {
	port := 7788 + GinkgoParallelNode()

	volumeDir, err := ioutil.TempDir("", fmt.Sprintf("baggageclaim_volume_dir_%d", GinkgoParallelNode()))
	Expect(err).NotTo(HaveOccurred())

	err = os.Mkdir(filepath.Join(volumeDir, "overlays"), 0700)
	Expect(err).NotTo(HaveOccurred())

	return &BaggageClaimRunner{
		path:      path,
		port:      port,
		volumeDir: volumeDir,
		driver:    driver,
	}
}

func (bcr *BaggageClaimRunner) Start() {
	config := baggageclaimcmd.BaggageclaimConfig{
		BindPort: uint16(bcr.port),
		Debug: baggageclaimcmd.DebugConfig{
			BindPort: uint16(8099 + GinkgoParallelNode()),
		},
		VolumesDir:  flag.Dir(bcr.volumeDir),
		Driver:      bcr.driver,
		OverlaysDir: filepath.Join(bcr.volumeDir, "overlays"),
	}

	configYAML, err := yaml.Marshal(config)
	Expect(err).ToNot(HaveOccurred())

	configFile, err := ioutil.TempFile("", "config.yml")
	Expect(err).NotTo(HaveOccurred())

	defer configFile.Close()

	_, err = configFile.Write(configYAML)
	Expect(err).NotTo(HaveOccurred())

	runner := ginkgomon.New(ginkgomon.Config{
		Name: "baggageclaim",
		Command: exec.Command(
			bcr.path,
			"--config", configFile.Name(),
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
	Expect(err).NotTo(HaveOccurred())
}

func (bcr *BaggageClaimRunner) Client() baggageclaim.Client {
	return client.New(fmt.Sprintf("http://localhost:%d", bcr.port), &http.Transport{DisableKeepAlives: true})
}

func (bcr *BaggageClaimRunner) VolumeDir() string {
	return bcr.volumeDir
}

func (bcr *BaggageClaimRunner) Port() int {
	return bcr.port
}

func (bcr *BaggageClaimRunner) CurrentHandles() []string {
	volumes, err := bcr.Client().ListVolumes(logger, nil)
	Expect(err).NotTo(HaveOccurred())

	handles := []string{}

	for _, v := range volumes {
		handles = append(handles, v.Handle())
	}

	return handles
}

func writeData(volumePath string) string {
	filename := randSeq(10)
	newFilePath := filepath.Join(volumePath, filename)

	err := ioutil.WriteFile(newFilePath, []byte(filename), 0755)
	Expect(err).NotTo(HaveOccurred())

	return filename
}

func randSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func dataExistsInVolume(filename, volumePath string) bool {
	_, err := os.Stat(filepath.Join(volumePath, filename))
	return err == nil
}
