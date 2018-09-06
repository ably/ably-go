package ably_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ably/ably-go/ably"
	"github.com/ably/ably-go/ably/ablytest"
	"github.com/ably/ably-go/ably/internal/ablyutil"
	"github.com/ably/ably-go/ably/proto"

	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/ably/ably-go/Godeps/_workspace/src/github.com/onsi/gomega"
)

func newHTTPClientMock(srv *httptest.Server) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) { return url.Parse(srv.URL) },
		},
	}
}

var _ = Describe("RestClient", func() {
	var (
		server *httptest.Server
	)
	Context("with a failing request", func() {
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

			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("encoding messages", func() {
		var (
			buffer   []byte
			server   *httptest.Server
			client   *ably.RestClient
			mockType string
			mockBody []byte
			err      error
		)

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				buffer, err = ioutil.ReadAll(r.Body)
				Expect(err).NotTo(HaveOccurred())

				w.Header().Set("Content-Type", mockType)
				w.WriteHeader(200)
				w.Write(mockBody)
			}))

		})

		Context("with JSON encoding set up", func() {
			BeforeEach(func() {
				options := &ably.ClientOptions{
					NoTLS:            true,
					NoBinaryProtocol: true,
					HTTPClient:       newHTTPClientMock(server),
					AuthOptions: ably.AuthOptions{
						UseTokenAuth: true,
					},
				}

				mockType = "application/json"
				mockBody = []byte("{}")

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
					HTTPClient: newHTTPClientMock(server),
					AuthOptions: ably.AuthOptions{
						UseTokenAuth: true,
					},
				}

				mockType = "application/x-msgpack"
				mockBody = []byte{0x80}

				options = testApp.Options(options)
				options.NoBinaryProtocol = false
				client, err = ably.NewRestClient(options)
				Expect(err).NotTo(HaveOccurred())

				err := client.Channel("test").Publish("ping", "pong")
				Expect(err).NotTo(HaveOccurred())
			})

			It("encode the body of the message using msgpack", func() {
				var anyMsgPack []map[string]interface{}
				err := ablyutil.Unmarshal(buffer, &anyMsgPack)
				Expect(err).NotTo(HaveOccurred())
				Expect(anyMsgPack[0]["name"]).To(Equal([]byte("ping")))
				Expect(anyMsgPack[0]["data"]).To(Equal([]byte("pong")))
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
		var stats []*proto.Stats

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

			stats[0].IntervalID = proto.IntervalFormatFor(lastInterval.Add(-120*time.Minute), proto.StatGranularityMinute)
			stats[1].IntervalID = proto.IntervalFormatFor(lastInterval.Add(-60*time.Minute), proto.StatGranularityMinute)
			stats[2].IntervalID = proto.IntervalFormatFor(lastInterval.Add(-1*time.Minute), proto.StatGranularityMinute)

			res, err := client.Post("/stats", &stats, nil)
			Expect(err).NotTo(HaveOccurred())
			res.Body.Close()
		})

		It("parses stats from the rest api", func() {
			longAgo := lastInterval.Add(-120 * time.Minute)
			page, err := client.Stats(&ably.PaginateParams{
				Limit: 1,
				ScopeParams: ably.ScopeParams{
					Start: ably.Time(longAgo),
					Unit:  proto.StatGranularityMinute,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(page.Stats()[0].IntervalID).To(MatchRegexp("[0-9]+\\-[0-9]+\\-[0-9]+:[0-9]+:[0-9]+"))
		})
	})
})

func TestRSC7(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	c, err := ably.NewRestClient(app.Options())
	if err != nil {
		t.Fatal(err)
	}
	t.Run("must set version header", func(ts *testing.T) {
		req, err := c.NewHTTPRequest(&ably.Request{})
		if err != nil {
			ts.Fatal(err)
		}
		h := req.Header.Get(ably.AblyVersionHeader)
		if h != ably.AblyVersion {
			t.Errorf("expected %s got %s", ably.AblyVersion, h)
		}
	})
	t.Run("must set lib header", func(ts *testing.T) {
		req, err := c.NewHTTPRequest(&ably.Request{})
		if err != nil {
			ts.Fatal(err)
		}
		h := req.Header.Get(ably.AblyLibHeader)
		if h != ably.LibraryString {
			t.Errorf("expected %s got %s", ably.LibraryString, h)
		}
	})
}

func TestRest_hostfallback(t *testing.T) {
	app, err := ablytest.NewSandbox(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	t.Run("RSC15d must use alternative host", func(ts *testing.T) {
		var retryCount int
		var hosts []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if retryCount > 0 {
				hosts = append(hosts, r.Host)
			}
			retryCount++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		options := &ably.ClientOptions{
			NoTLS:      true,
			HTTPClient: newHTTPClientMock(server),
			AuthOptions: ably.AuthOptions{
				UseTokenAuth: true,
			},
		}
		client, err := ably.NewRestClient(app.Options(options))
		if err != nil {
			ts.Fatal(err)
		}
		err = client.Channel("test").Publish("ping", "pong")
		if err == nil {
			ts.Error("expected an error")
		}
		if retryCount != 4 {
			t.Errorf("expected 4 retries got %d", retryCount)
		}
		// make sure the host header is set. Since we are using defaults from the spec
		// the hosts should be in [a..e].ably-realtime.com
		expect := ably.DefaultFallbackHosts()
		for _, host := range hosts {
			if sort.SearchStrings(expect, host) == -1 {
				t.Errorf("unexpected host %s", host)
			}
		}
	})
}
