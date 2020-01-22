package driver_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type LiveVolume struct {
	Path   string
	Parent string
}

func VolumeTree(root string) ([]LiveVolume, error) {
	dirs, err := ioutil.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	vols := make([]LiveVolume, len(dirs))
	for idx, dir := range dirs {
		vols[idx] = LiveVolume{
			Path: filepath.Join(root, dir.Name()),
		}

		parentTarget, err := readlink(filepath.Join(root, dir.Name(), "parent"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return nil, err
		}

		vols[idx].Parent = parentTarget
	}

	return vols, nil
}

func MountOrder(vols []LiveVolume) []LiveVolume {
	// TODO
	return vols
}

func readlink(path string) (target string, err error) {
	finfo, err := os.Lstat(path)
	if err != nil {
		return
	}

	if finfo.Mode()&os.ModeSymlink != os.ModeSymlink {
		err = fmt.Errorf("not a symlink")
		return
	}

	target, err = os.Readlink(path)
	return
}

var _ = FDescribe("overlay", func() {

	Describe("MountOrder", func() {

		type Case struct {
			input, expected []LiveVolume
		}

		DescribeTable("scenarios",
			func(c Case) {
				actual := MountOrder(c.input)
				Expect(actual).To(Equal(c.expected))
			},
			Entry("no vols", Case{
				input:    nil,
				expected: nil,
			}),
			Entry("single vol", Case{
				input:    []LiveVolume{{Path: "vol1"}},
				expected: []LiveVolume{{Path: "vol1"}},
			}),
			Entry("vols without relationships", Case{
				input:    []LiveVolume{{Path: "vol1"}, {Path: "vol2"}},
				expected: []LiveVolume{{Path: "vol1"}, {Path: "vol2"}},
			}),
			Entry("having relationship, orders by dependency", Case{
				input: []LiveVolume{
					{Path: "vol1", Parent: "vol2"},
					{Path: "vol2"},
				},
				expected: []LiveVolume{
					{Path: "vol2"},
					{Path: "vol1", Parent: "vol2"},
				},
			}),
		)

	})

	Describe("VolumeTree", func() {

		var (
			vols []LiveVolume
			root string
			err  error
		)

		JustBeforeEach(func() {
			vols, err = VolumeTree(root)
		})

		Context("inexistent live root", func() {
			It("does not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns no vols", func() {
				Expect(vols).To(BeEmpty())
			})
		})

		Context("existing live root", func() {

			BeforeEach(func() {
				root, err = ioutil.TempDir("", "bclaim_ovl_test")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				os.RemoveAll(root)
			})

			Context("having no volumes", func() {
				It("does not error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns no vols", func() {
					Expect(vols).To(BeEmpty())
				})
			})

			Context("having volumes", func() {

				BeforeEach(func() {
					err = os.Mkdir(filepath.Join(root, "vol1"), 0755)
					Expect(err).ToNot(HaveOccurred())

					err = os.Mkdir(filepath.Join(root, "vol2"), 0755)
					Expect(err).ToNot(HaveOccurred())
				})

				It("finds them", func() {
					Expect(vols).To(HaveLen(2))
					Expect(vols[0].Parent).To(BeEmpty())
					Expect(vols[1].Parent).To(BeEmpty())
				})

				Context("having malformed relationship", func() {
					BeforeEach(func() {
						_, err = os.Create(filepath.Join(root, "vol2", "parent"))
						Expect(err).ToNot(HaveOccurred())
					})

					It("fails", func() {
						Expect(err).To(HaveOccurred())
					})
				})

				Context("having relationships", func() {
					BeforeEach(func() {
						err = os.Symlink(
							filepath.Join(root, "vol1"),
							filepath.Join(root, "vol2", "parent"),
						)
						Expect(err).ToNot(HaveOccurred())
					})

					It("suceeds", func() {
						Expect(err).ToNot(HaveOccurred())
					})

					It("discovers the relationship", func() {
						Expect(vols).To(HaveLen(2))
						Expect(vols[1].Parent).ToNot(BeEmpty())
						Expect(vols[1].Parent).To(Equal(vols[0].Path))
					})
				})
			})
		})
	})

})
