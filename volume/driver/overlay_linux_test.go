package driver_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/driver"
	"github.com/concourse/baggageclaim/volume/driver/driverfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Overlay", func() {
	Describe("Driver", func() {
		var tmpdir string
		var fs volume.Filesystem

		BeforeEach(func() {
			var err error
			tmpdir, err = ioutil.TempDir("", "overlay-test")
			Expect(err).ToNot(HaveOccurred())

			volumesDir := filepath.Join(tmpdir, "volumes")
			overlaysDir := filepath.Join(tmpdir, "overlays")

			overlayDriver, err := driver.NewOverlayDriver(volumesDir, overlaysDir)
			Expect(err).ToNot(HaveOccurred())

			fs, err = volume.NewFilesystem(overlayDriver, volumesDir)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpdir)).To(Succeed())
		})

		It("supports nesting >2 levels deep", func() {
			rootVolInit, err := fs.NewVolume("root-vol")
			Expect(err).ToNot(HaveOccurred())

			// write to file under rootVolData
			rootFile := filepath.Join(rootVolInit.DataPath(), "rootFile")
			err = ioutil.WriteFile(rootFile, []byte("root"), 0644)
			Expect(err).ToNot(HaveOccurred())

			doomedFile := filepath.Join(rootVolInit.DataPath(), "doomedFile")
			err = ioutil.WriteFile(doomedFile, []byte("im doomed"), 0644)
			Expect(err).ToNot(HaveOccurred())

			rootVolLive, err := rootVolInit.Initialize()
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				err := rootVolLive.Destroy()
				Expect(err).ToNot(HaveOccurred())
			}()

			childVolInit, err := rootVolLive.NewSubvolume("child-vol")
			Expect(err).ToNot(HaveOccurred())

			// write to file under rootVolData
			chileFilePath := filepath.Join(childVolInit.DataPath(), "rootFile")
			err = ioutil.WriteFile(chileFilePath, []byte("child"), 0644)
			Expect(err).ToNot(HaveOccurred())

			err = os.Remove(filepath.Join(childVolInit.DataPath(), "doomedFile"))
			Expect(err).ToNot(HaveOccurred())

			childVolLive, err := childVolInit.Initialize()
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				err := childVolLive.Destroy()
				Expect(err).ToNot(HaveOccurred())
			}()

			childVol2Init, err := childVolLive.NewSubvolume("child-vol-2")
			Expect(err).ToNot(HaveOccurred())

			childVol2Live, err := childVol2Init.Initialize()
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				err := childVol2Live.Destroy()
				Expect(err).ToNot(HaveOccurred())
			}()

			childVol3Init, err := childVol2Live.NewSubvolume("child-vol-3")
			Expect(err).ToNot(HaveOccurred())

			childVol3Live, err := childVol3Init.Initialize()
			Expect(err).ToNot(HaveOccurred())

			defer func() {
				err := childVol3Live.Destroy()
				Expect(err).ToNot(HaveOccurred())
			}()

			child3FilePath := filepath.Join(childVol3Live.DataPath(), "rootFile")
			content, err := ioutil.ReadFile(child3FilePath)
			Expect(string(content)).To(Equal("child"))

			_, err = os.Stat(filepath.Join(childVol3Live.DataPath(), "doomedFile"))
			Expect(err).To(HaveOccurred())
		})
	})

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

			// this is technically a valid case to test given the types, however
			// in the real world we prevent two levels of nesting.
			//
			// see https://github.com/concourse/concourse/issues/5799
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
