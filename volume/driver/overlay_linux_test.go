package driver_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/baggageclaim/volume/driver"
	"github.com/concourse/baggageclaim/volume/driver/driverfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Overlay", func() {

	Describe("Ancestry", func() {

		type Case struct {
			input    driver.LiveVolume
			expected []driver.LiveVolume
		}

		DescribeTable("cases",
			func(c Case) {
				Expect(driver.Ancestry(c.input)).To(Equal(c.expected))
			},
			Entry("orphan", Case{
				input: driver.LiveVolume{Path: "vol1"},
				expected: []driver.LiveVolume{
					{Path: "vol1"},
				},
			}),
			Entry("w/ parent", Case{
				input: driver.LiveVolume{
					Path: "vol1",
					Parent: &driver.LiveVolume{
						Path: "vol2",
					},
				},
				expected: []driver.LiveVolume{
					{Path: "vol2"},
					{Path: "vol1", Parent: &driver.LiveVolume{Path: "vol2"}},
				},
			}),
			Entry("w/ granparent", Case{
				input: driver.LiveVolume{
					Path: "vol1",
					Parent: &driver.LiveVolume{
						Path: "vol2",
						Parent: &driver.LiveVolume{
							Path: "vol3",
						},
					},
				},
				expected: []driver.LiveVolume{
					{Path: "vol3"},
					{Path: "vol2", Parent: &driver.LiveVolume{Path: "vol3"}},
					{Path: "vol1", Parent: &driver.LiveVolume{Path: "vol2", Parent: &driver.LiveVolume{Path: "vol3"}}},
				},
			}),
		)

	})

	Describe("RecoverMountTable", func() {

		var (
			root    string
			err     error
			mounter *driverfakes.FakeMounter
		)

		JustBeforeEach(func() {
			err = driver.RecoverMountTable(root, mounter)
		})

		BeforeEach(func() {
			mounter = new(driverfakes.FakeMounter)
		})

		Context("not having a live dir", func() {
			It("does nothing", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("having a live dir", func() {

			BeforeEach(func() {
				root, err = ioutil.TempDir("", "bclaim_ovl_test")
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				os.RemoveAll(root)
			})

			Context("having no volumes", func() {
				It("succeeds", func() {
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("having a file in the middle of vol dirs", func() {
				BeforeEach(func() {
					_, err = os.Create(filepath.Join(root, "file"))
					Expect(err).ToNot(HaveOccurred())
				})

				It("fails", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("having volume w/out parents", func() {
				BeforeEach(func() {
					err = os.Mkdir(filepath.Join(root, "vol1"), 0755)
					Expect(err).ToNot(HaveOccurred())
				})

				It("succeeds", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("tries to bind mounts volumes without parents", func() {
					Expect(mounter.BindMountCallCount()).To(Equal(1))
					Expect(mounter.BindMountArgsForCall(0)).To(Equal(filepath.Join(root, "vol1", "volume")))
				})

				Context("with bind mount failing", func() {
					BeforeEach(func() {
						mounter.BindMountReturns(errors.New("bind-mount-err"))
					})

					It("fails", func() {
						Expect(err).To(HaveOccurred())
					})
				})

				// root
				//   vol1
				//   vol2
				//      parent --> vol1
				//
				Context("having a parental relationship", func() {
					BeforeEach(func() {
						err = os.Mkdir(filepath.Join(root, "vol2"), 0755)
						Expect(err).ToNot(HaveOccurred())

						err = os.Symlink(
							filepath.Join(root, "vol1"),
							filepath.Join(root, "vol2", "parent"),
						)
						Expect(err).ToNot(HaveOccurred())
					})

					It("tries to perform overlay mount for child", func() {
						Expect(mounter.OverlayMountCallCount()).To(Equal(1))
						mountpoint, parent := mounter.OverlayMountArgsForCall(0)

						Expect(mountpoint).To(Equal(filepath.Join(root, "vol2", "volume")))
						Expect(parent).To(Equal(filepath.Join(root, "vol1", "volume")))
					})

					Context("having overlaymount failing", func() {
						BeforeEach(func() {
							mounter.OverlayMountReturns(errors.New("overlay-mount-err"))
						})

						It("errors", func() {
							Expect(err).To(HaveOccurred())
						})
					})
				})

				// root
				//   vol1
				//   vol2
				//      parent --> vol1
				//   vol3
				//      parent --> vol1
				//
				Context("having multiple children depending on same parent", func() {
					BeforeEach(func() {
						err = os.Mkdir(filepath.Join(root, "vol3"), 0755)
						Expect(err).ToNot(HaveOccurred())

						err = os.Symlink(
							filepath.Join(root, "vol1"),
							filepath.Join(root, "vol3", "parent"),
						)
						Expect(err).ToNot(HaveOccurred())

					})

					It("does not perform bind mount twice", func() {
						Expect(mounter.BindMountCallCount()).To(Equal(1))
					})
				})
			})
		})
	})

	Describe("Newdriver.LiveVolume", func() {

		var (
			res       *driver.LiveVolume
			root, vol string
			err       error
		)

		JustBeforeEach(func() {
			res, err = driver.NewLiveVolume(root, vol)
		})

		Context("with inexistent root", func() {
			It("fails", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with root existing", func() {
			BeforeEach(func() {
				root, err = ioutil.TempDir("", "bclaim_ovl_test")
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				os.RemoveAll(root)
			})

			Context("volume not existing", func() {
				BeforeEach(func() {
					vol = "inexistent"
				})

				It("fails", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("volume existing", func() {
				BeforeEach(func() {
					vol = "vol1"
					err = os.Mkdir(filepath.Join(root, vol), 0755)
					Expect(err).ToNot(HaveOccurred())
				})

				It("succeeds", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("records the path", func() {
					Expect(res).ToNot(BeNil())
					Expect(res.Path).To(Equal(filepath.Join(root, "vol1")))
				})

				It("doesn't fill parent", func() {
					Expect(res.Parent).To(BeNil())
				})

				Context("having a malformed parent indicator", func() {
					BeforeEach(func() {
						_, err = os.Create(filepath.Join(root, vol, "parent"))
						Expect(err).ToNot(HaveOccurred())
					})

					It("fails", func() {
						Expect(err).To(HaveOccurred())
					})
				})

				Context("having a proper parent", func() {
					BeforeEach(func() {
						err = os.Mkdir(filepath.Join(root, "parent-vol"), 0755)
						Expect(err).ToNot(HaveOccurred())

						err = os.Symlink(
							filepath.Join(root, "parent-vol"),
							filepath.Join(root, vol, "parent"),
						)
						Expect(err).ToNot(HaveOccurred())
					})

					It("succeeds", func() {
						Expect(err).ToNot(HaveOccurred())
					})

					It("fills Parent", func() {
						Expect(res.Parent).To(Equal(&driver.LiveVolume{
							Path:   filepath.Join(root, "parent-vol"),
							Parent: nil,
						}))
					})

					Context("which has a parent itself", func() {
						BeforeEach(func() {
							err = os.Mkdir(filepath.Join(root, "parent-parent-vol"), 0755)
							Expect(err).ToNot(HaveOccurred())

							err = os.Symlink(
								filepath.Join(root, "parent-parent-vol"),
								filepath.Join(root, "parent-vol", "parent"),
							)
							Expect(err).ToNot(HaveOccurred())
						})

						It("succeeds", func() {
							Expect(err).ToNot(HaveOccurred())
						})

						It("fills both parent and parent's parent", func() {
							Expect(res.Parent).To(Equal(&driver.LiveVolume{
								Path: filepath.Join(root, "parent-vol"),
								Parent: &driver.LiveVolume{
									Path:   filepath.Join(root, "parent-parent-vol"),
									Parent: nil,
								},
							}))
						})
					})
				})
			})
		})
	})

})
