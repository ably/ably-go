package ably_test

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

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/proto"

	"github.com/ably/ably-go/Godeps/_workspace/src/gopkg.in/vmihailenco/msgpack.v2"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

var _ = Describe("RestClient", func() {
	var (
		server *httptest.Server

		newHTTPClientMock = func(srv *httptest.Server) *http.Client {
			return &http.Client{
				Transport: &http.Transport{
					Proxy: func(*http.Request) (*url.URL, error) { return url.Parse(srv.URL) },
				},
			}
		}
	)

	Context("with a failing request", func() {
		var (
			request *http.Request
			client  *ably.RestClient
		)

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"message":"Not Found"}`)
			}))

			options := &ably.ClientOptions{NoTLS: true, HTTPClient: newHTTPClientMock(server)}

			var err error
			client, err = ably.NewRestClient(testApp.Options(options))
			Expect(err).NotTo(HaveOccurred())

			request, err = http.NewRequest("POST", options.RestURL()+"/any_path", bytes.NewBuffer([]byte{}))
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("Get", func() {
			var data interface{}

			It("fails with a meaningful error", func() {
				_, err := client.Get("/any_path", data)
				Expect(err).To(HaveOccurred())

				e, ok := err.(*ably.Error)
				Expect(ok).To(BeTrue())
				Expect(e.Err.Error()).To(Equal("Not Found"))
				Expect(e.StatusCode).To(Equal(404))
			})
		})

		Describe("Post", func() {
			It("fails with a meaningful error", func() {
				_, err := client.Post("/any_path", request, nil)
				Expect(err).To(HaveOccurred())

				e, ok := err.(*ably.Error)
				Expect(ok).To(BeTrue())
				Expect(e.Err.Error()).To(Equal("Not Found"))
				Expect(e.StatusCode).To(Equal(404))
			})
		})
	})

	Describe("encoding messages", func() {
		var (
			buffer []byte
			server *httptest.Server
			client *ably.RestClient
			err    error
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
				options := &ably.ClientOptions{
					NoTLS:      true,
					Protocol:   ably.ProtocolJSON,
					HTTPClient: newHTTPClientMock(server),
				}

				client, err = ably.NewRestClient(testApp.Options(options))
				Expect(err).NotTo(HaveOccurred())

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
				options := &ably.ClientOptions{
					NoTLS:      true,
					Protocol:   ably.ProtocolMsgPack,
					HTTPClient: newHTTPClientMock(server),
				}

				client, err = ably.NewRestClient(testApp.Options(options))
				Expect(err).NotTo(HaveOccurred())

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
		It("returns srv time", func() {
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
			page, err := client.Stats(&ably.PaginateParams{
				Limit: 1,
				ScopeParams: ably.ScopeParams{
					Start: ably.Timestamp(longAgo),
					Unit:  proto.StatGranularityMinute,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(page.Stats()[0].IntervalId).To(MatchRegexp("[0-9]+\\-[0-9]+\\-[0-9]+:[0-9]+:[0-9]+"))
		})
	})
})
