package integration_test

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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
