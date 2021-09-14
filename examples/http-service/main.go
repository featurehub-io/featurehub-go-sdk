package main

import (
	"net/http"

	client "github.com/featurehub-io/featurehub-go-sdk"
	"github.com/featurehub-io/featurehub-go-sdk/examples/http-service/internal/handler"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/analytics"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	listenAddress = ":8080"
	logLevel      = logrus.TraceLevel
	sdkKey        = "default/8c397b3e-6859-4d8e-b159-8aaa7612a15e/zdq34UE8I9qQzozdnHJxbAXfMNiXXV*GGVdiQoomNPKCchWWmMz"
	serverAddress = "http://localhost:8085"
)

func main() {

	// Set up a TRACE level logger:
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)

	// Prepare a config:
	fhConfig, err := client.New(serverAddress, sdkKey).WithLogLevel(logLevel).WithWaitForData(true).Connect()
	if err != nil {
		logrus.Fatalf("Error creating config")
	}

	fhClient := fhConfig.NewContext()

	// Configure a logging analytics collector:
	fhClient.AddAnalyticsCollector(analytics.NewLoggingAnalyticsCollector(logger))

	// Prepare a turn.io handler using the recorder:
	handler := handler.New(logger, fhClient)

	// Prepare a MUX router:
	router := mux.NewRouter()
	router.HandleFunc("/mapped", handler.Mapped).Methods(http.MethodGet)
	router.HandleFunc("/random", handler.Random).Methods(http.MethodGet)
	router.HandleFunc("/static", handler.Static).Methods(http.MethodGet)
	http.Handle("/", router)

	// Serve:
	logrus.WithField("listen_address", listenAddress).Info("Started serving")
	logrus.WithError(http.ListenAndServe(listenAddress, router)).Fatal("Stopped serving")
}
