package rest_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/ably/ably-go/rest"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	Context("with a failing request", func() {
		var (
			statusCode int
			request    *http.Request
		)

		BeforeEach(func() {
			statusCode = 404
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"error":"Not Found"}`)
			}))

			client.RestEndpoint = strings.Replace(client.RestEndpoint, "https", "http", 1)

			client.HttpClient = &http.Client{
				Transport: &http.Transport{
					Proxy: func(req *http.Request) (*url.URL, error) {
						return url.Parse(server.URL)
					},
				},
			}

			var err error
			request, err = http.NewRequest("POST", client.RestEndpoint+"/any_path", bytes.NewBuffer([]byte{}))
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("Get", func() {
			var data interface{}
			It("fails with a meaningful error", func() {
				_, err := client.Get("/any_path", data)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Unexpected status code 404"))

				httpError, ok := err.(*rest.RestHttpError)
				Expect(ok).To(BeTrue())
				Expect(httpError.ResponseBody).To(Equal(`{"error":"Not Found"}`))
			})
		})

		Describe("Post", func() {
			It("fails with a meaningful error", func() {
				_, err := client.Post("/any_path", request, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Unexpected status code 404"))

				httpError, ok := err.(*rest.RestHttpError)
				Expect(ok).To(BeTrue())
				Expect(httpError.ResponseBody).To(Equal(`{"error":"Not Found"}`))
			})
		})
	})

	Describe("RequestToken", func() {
		It("gets a token from the API", func() {
			ttl := 60 * 60
			capability := &rest.Capability{"foo": []string{"publish"}}
			token, err := client.Auth.RequestToken(ttl, capability)

			Expect(err).NotTo(HaveOccurred())
			Expect(token.ID).To(ContainSubstring(testApp.Config.AppID))
			Expect(token.Key).To(Equal(testApp.AppKeyId()))
			Expect(token.Capability).To(Equal(capability))
		})
	})

	Describe("Time", func() {
		It("returns server time", func() {
			t, err := client.Time()
			Expect(err).NotTo(HaveOccurred())
			Expect(t.Unix()).To(BeNumerically("<=", time.Now().Unix()))
			Expect(t.Unix()).To(BeNumerically(">=", time.Now().Add(-2*time.Second).Unix()))
		})
	})
})
