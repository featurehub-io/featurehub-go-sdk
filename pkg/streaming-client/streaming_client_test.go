package streamingclient

import (
	"bytes"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// testEvent implements the simple "Event" interface from donovanhide/eventsource:
type testEvent struct {
	data  string
	event string
	id    string
}

func (e *testEvent) Data() string  { return e.data }
func (e *testEvent) Event() string { return e.event }
func (e *testEvent) Id() string    { return e.id }

func TestStreamingClient(t *testing.T) {
	timeout := time.Hour
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logBuffer := new(bytes.Buffer)
	logger.SetOutput(logBuffer)

	// Make a test config (with an incorrect server address):
	config := &core.Config{
		LogLevel:      logrus.FatalLevel,
		Logger:        logger,
		SDKKey:        "environment-id/my-secret-api-key",
		ServerAddress: "http://streams.test:8086",
		WaitForData:   &timeout,
	}

	// Attempt to make a new client (config has a non-existent hostname):
	client, err := NewStreamingClient(config, core.NewClientFeatureHubRepository(logger))
	assert.Error(t, err)
	assert.Implements(t, new(interfaces.EdgeClient), client)
}
