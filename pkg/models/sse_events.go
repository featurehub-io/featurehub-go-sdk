package models

// Event is a type which allows us to define the different SSE-events we can expect from a FeatureHub server:
type Event string

// SSEAck is a standard SSE event (a positive confirmation):
const SSEAck Event = "ack"

// SSEBye is a standard SSE event (connection ending):
const SSEBye Event = "bye"

// SSEError is a standard SSE event (connection error):
const SSEError Event = "error"

// FHConfig is a FeatureHub config event (client configuration instructions from the server):
const FHConfig Event = "config"

// FHDeleteFeature is a FeatureHub SSE event (telling us that a feature has been deleted):
const FHDeleteFeature Event = "delete_feature"

// FHFailure is a FeatureHub SSE event (server-side error):
const FHFailure Event = "failure"

// FHFeature is a FeauterHub SSE event (an update to a specific feature):
const FHFeature Event = "feature"

// FHFeatures is a FeauterHub SSE event (an entire feature set):
const FHFeatures Event = "features"
