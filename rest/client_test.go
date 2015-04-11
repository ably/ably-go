package rest_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/ably/ably-go"
	"github.com/ably/ably-go/Godeps/_workspace/src/gopkg.in/vmihailenco/msgpack.v2"
	"github.com/ably/ably-go/config"
	"github.com/ably/ably-go/proto"
	"github.com/ably/ably-go/rest"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	var (
		server *httptest.Server

		createMockedClient = func(server *httptest.Server) *http.Client {
			return &http.Client{
				Transport: &http.Transport{
					Proxy: func(req *http.Request) (*url.URL, error) {
						return url.Parse(server.URL)
					},
				},
			}
		}
	)

	Context("with a failing request", func() {
		var (
			statusCode int
			request    *http.Request
		)

		BeforeEach(func() {
			statusCode = 404
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"error":"Not Found"}`)
			}))

			client.RestEndpoint = strings.Replace(client.RestEndpoint, "https", "http", 1)
			client.HTTPClient = createMockedClient(server)

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

	Describe("encoding messages", func() {
		var (
			buffer []byte
			server *httptest.Server
		)

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				buffer, err = ioutil.ReadAll(r.Body)
				Expect(err).NotTo(HaveOccurred())

				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{}`)
			}))
		})

		Context("with JSON encoding set up", func() {
			BeforeEach(func() {
				testParamsCopy := testApp.Params
				testParamsCopy.Protocol = config.ProtocolJSON
				client = ably.NewRestClient(testParamsCopy)

				client.RestEndpoint = strings.Replace(client.RestEndpoint, "https", "http", 1)
				client.HTTPClient = createMockedClient(server)

				err := client.Channel("test").Publish("ping", "pong")
				Expect(err).NotTo(HaveOccurred())
			})

			It("encode the body of the message in JSON", func() {
				var anyJson []map[string]interface{}
				err := json.Unmarshal(buffer, &anyJson)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with msgpack encoding set up", func() {
			BeforeEach(func() {
				testParamsCopy := testApp.Params
				testParamsCopy.Protocol = config.ProtocolMsgPack
				client = ably.NewRestClient(testParamsCopy)

				client.RestEndpoint = strings.Replace(client.RestEndpoint, "https", "http", 1)
				client.HTTPClient = createMockedClient(server)

				err := client.Channel("test").Publish("ping", "pong")
				Expect(err).NotTo(HaveOccurred())
			})

			It("encode the body of the message using msgpack", func() {
				var anyMsgPack []map[string]interface{}
				err := msgpack.Unmarshal(buffer, &anyMsgPack)
				Expect(err).NotTo(HaveOccurred())
				Expect(anyMsgPack[0]["name"]).To(Equal("ping"))
				Expect(anyMsgPack[0]["data"]).To(Equal("pong"))
			})
		})
	})

	Describe("Time", func() {
		It("returns server time", func() {
			t, err := client.Time()
			Expect(err).NotTo(HaveOccurred())
			Expect(t.Unix()).To(BeNumerically("<=", time.Now().Add(2*time.Second).Unix()))
			Expect(t.Unix()).To(BeNumerically(">=", time.Now().Add(-2*time.Second).Unix()))
		})
	})

	Describe("Stats", func() {
		var lastInterval = time.Now().Add(-365 * 24 * time.Hour)
		var stats []*proto.Stat

		var jsonStats = `
			[
				{
					"inbound":{"realtime":{"messages":{"count":50,"data":5000}}},
					"outbound":{"realtime":{"messages":{"count":20,"data":2000}}}
				},
				{
					"inbound":{"realtime":{"messages":{"count":60,"data":6000}}},
					"outbound":{"realtime":{"messages":{"count":10,"data":1000}}}
				},
				{
					"inbound":{"realtime":{"messages":{"count":70,"data":7000}}},
					"outbound":{"realtime":{"messages":{"count":40,"data":4000}}},
					"persisted":{"presence":{"count":20,"data":2000}},
					"connections":{"tls":{"peak":20,"opened":10}},
					"channels":{"peak":50,"opened":30},
					"apiRequests":{"succeeded":50,"failed":10},
					"tokenRequests":{"succeeded":60,"failed":20}
				}
			]
		`

		BeforeEach(func() {
			err := json.NewDecoder(strings.NewReader(jsonStats)).Decode(&stats)
			Expect(err).NotTo(HaveOccurred())

			stats[0].IntervalId = proto.IntervalFormatFor(lastInterval.Add(-120*time.Minute), proto.StatGranularityMinute)
			stats[1].IntervalId = proto.IntervalFormatFor(lastInterval.Add(-60*time.Minute), proto.StatGranularityMinute)
			stats[2].IntervalId = proto.IntervalFormatFor(lastInterval.Add(-1*time.Minute), proto.StatGranularityMinute)

			res, err := client.Post("/stats", &stats, nil)
			Expect(err).NotTo(HaveOccurred())
			res.Body.Close()
		})

		It("parses stats from the rest api", func() {
			longAgo := lastInterval.Add(-120 * time.Minute)
			paginatedStats, err := client.Stats(&config.PaginateParams{
				Limit: 1,
				ScopeParams: config.ScopeParams{
					Start: config.NewTimestamp(longAgo),
					Unit:  proto.StatGranularityMinute,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(paginatedStats.Current[0].IntervalId).To(MatchRegexp("[0-9]+\\-[0-9]+\\-[0-9]+:[0-9]+:[0-9]+"))
		})
	})
})
