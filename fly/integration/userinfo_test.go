package integration_test

import (
	"os/exec"

	"github.com/concourse/concourse/fly/ui"
	"github.com/fatih/color"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Fly CLI", func() {
	Describe("userinfo", func() {
		var (
			flyCmd *exec.Cmd
		)

		BeforeEach(func() {
			flyCmd = exec.Command(flyPath, "-t", targetName, "userinfo")
		})

		Context("when userinfo is returned from the API", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/user"),
						ghttp.RespondWithJSONEncoded(200, map[string]any{
							"sub":       "zero",
							"name":      "test",
							"user_id":   "test_id",
							"user_name": "test_user",
							"email":     "test_email",
							"is_admin":  false,
							"is_system": false,
							"teams": map[string][]string{
								"other_team": {"owner"},
								"test_team":  {"owner", "viewer"},
							},
							"connector":       "some-connector",
							"display_user_id": "test_id",
						}),
					),
				)
			})

			It("shows username", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(0))
				Expect(sess.Out).To(PrintTable(ui.Table{
					Headers: ui.TableRow{
						{Contents: "username", Color: color.New(color.Bold)},
						{Contents: "team/role", Color: color.New(color.Bold)},
					},
					Data: []ui.TableRow{
						{{Contents: "test_id"}, {Contents: "other_team/owner,test_team/owner,test_team/viewer"}},
					},
				}))
			})

			Context("when --json is given", func() {
				BeforeEach(func() {
					flyCmd.Args = append(flyCmd.Args, "--json")
				})

				It("prints response in json as stdout", func() {
					sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())

					Eventually(sess).Should(gexec.Exit(0))
					Expect(sess.Out.Contents()).To(MatchJSON(`{
							"sub":       "zero",
							"name":      "test",
							"user_id":   "test_id",
							"user_name": "test_user",
							"email":     "test_email",
							"is_admin":  false,
							"is_system": false,
							"teams": {
								"other_team": ["owner"],
								"test_team": ["owner", "viewer"]
							},
							"connector": "some-connector",
							"display_user_id": "test_id"
					}`))
				})
			})
		})

		Context("and the api returns an internal server error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v1/user"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("writes an error message to stderr", func() {
				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Eventually(sess).Should(gexec.Exit(1))
				Eventually(sess.Err).Should(gbytes.Say("Unexpected Response"))
			})
		})
	})
})
