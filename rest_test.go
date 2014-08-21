package ably

import (
	"fmt"
	"log"
	"testing"

	. "gopkg.in/check.v1"
)

type testAppNamespace struct {
	ID        string `json:"id"`
	Persisted bool   `json:"persisted"`
}

type testAppKey struct {
	ID    string `json:"id,omitempty"`
	Value string `json:"value,omitempty"`
}

type testApp struct {
	ID         string             `json:"id,omitempty"`
	Keys       []testAppKey       `json:"keys"`
	Namespaces []testAppNamespace `json:"namespaces"`
}

func (t *testApp) AppID() string {
	return fmt.Sprintf("%s.%s", t.ID, t.Keys[0].ID)
}

func (t *testApp) AppSecret() string {
	return t.Keys[0].Value
}

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type TestSuite struct {
	testApp *testApp
	client  *Client
}

var _ = Suite(&TestSuite{
	testApp: &testApp{
		Keys: []testAppKey{{}},
		Namespaces: []testAppNamespace{
			{ID: "persisted", Persisted: true},
		},
	},
	client: RestClient(Params{Endpoint: "https://sandbox-rest.ably.io"}),
})

func (s *TestSuite) SetUpSuite(c *C) {
	log.Println("Creating test app via", s.client.Endpoint)
	_, err := s.client.Post("/apps", s.testApp, s.testApp)
	if err != nil {
		c.Fatal(err)
	}
	log.Println("Got test app with ID", s.testApp.ID)
	s.client.AppID = s.testApp.AppID()
	s.client.AppSecret = s.testApp.AppSecret()
}

func (s *TestSuite) TearDownSuite(c *C) {
	if s.testApp.ID == "" {
		return
	}
	log.Println("Deleting test app with ID", s.testApp.ID)
	s.client.Delete("/apps/" + s.testApp.ID)
}

func (s *TestSuite) TestRestTime(c *C) {
	time, err := s.client.Time()
	if err != nil {
		c.Fatal(err)
	}
	c.Assert(time, NotNil)
}

func (s *TestSuite) TestRestPublish(c *C) {
	channel := s.client.Channel("test")
	err := channel.Publish(&Message{Name: "foo", Data: "woop!"})
	c.Assert(err, IsNil)
}
