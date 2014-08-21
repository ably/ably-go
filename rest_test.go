package ably

import (
	"fmt"
	"log"
	"testing"
	"time"

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

func (s *TestSuite) TestRestHistory(c *C) {
	channel := s.client.Channel("persisted")
	msgs := map[string]string{
		"Test1": "foo",
		"Test2": "bar",
		"Test3": "baz",
	}

	for name, data := range msgs {
		if err := channel.Publish(&Message{Name: name, Data: data}); err != nil {
			c.Fatalf("Failed to publish %s message: %s", name, err)
		}
	}

	time.Sleep(10 * time.Second)

	history, err := channel.History()
	c.Assert(err, IsNil)
	for _, msg := range history {
		data, ok := msgs[msg.Name]
		if !ok {
			c.Fatalf("Unexpected message %v", msg)
		}
		if data != msg.Data {
			c.Fatalf("Expected message %s to have value %s, got %s", msg.Name, data, msg.Data)
		}
	}
}
