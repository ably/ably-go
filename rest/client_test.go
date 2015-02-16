package rest_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/ably/ably-go"
	"github.com/ably/ably-go/rest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	Describe("RequestToken", func() {
		It("gets a token from the API", func() {
			ttl := 60 * 60
			capability := &ably.Capability{"foo": []string{"publish"}}
			token, err := client.RequestToken(ttl, capability)

			Expect(err).NotTo(HaveOccurred())
			Expect(token.ID).To(ContainSubstring(testApp.Config.AppID))
			Expect(token.Key).To(Equal(testApp.AppKeyId()))
			Expect(token.Capability).To(Equal(capability))
		})

		Context("with a failing request", func() {
			BeforeEach(func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(404)
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
			})

			It("has the body of the response available", func() {
				ttl := 60 * 60
				capability := &ably.Capability{"foo": []string{"publish"}}
				_, err := client.RequestToken(ttl, capability)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Unexpected status code 404"))

				httpError, ok := err.(*rest.RestHttpError)
				Expect(ok).To(BeTrue())
				Expect(httpError.ResponseBody()).To(Equal(`{"error":"Not Found"}`))
			})
		})
	})
})
