Example GoLang Service
======================

This is a simple example of a Go HTTP service using the FeatureHub Go SDK.
- Establishes a connection to a FeatureHub server
- Configures a "logging" AnalyticsCollector (events are simply emitted as logs instead of actually being forwarded to something like Google Analytics)
- Handles HTTP requests
- Submits events for each request


Usage
-----

First you need a FeatureHub server:
- Run a FeatureHub server (`docker run -d -p 8085:8085 --name featurehub -v $HOME/featurehub:/db featurehub/party-server:latest`)
- Configure it (http://localhost:8085)
- Add some features

Now run the example golang service:
- Put your API key into the consts in main.go
- `go run main.go`

Now hit the service with curl:
- `curl http://localhost:8080/static?name=somebody`
-You should see an analytics event logged to the console

Now try some more features:
- Add a boolean feature-flag called "goodbye"
- Watch the logs - you'll see the service pick up the new feature
- Hit the service with CURL again - it will now respond differently!

You can also try adding some custom rollout strategies to your "goodbye" feature:
- Add a split strategy for "userkey", and configure a few names that will cause it to return a differetn value
- Now hit the endpoint `curl http://localhost:8080/mapped?name=somebody` with some different names, and see the boolean value set accordingly

One final example here is how to use a percentage-based rollout strategy on the userkey field:
- Create a new boolean feature called "random" which defaults to false
- Add a percentage strategy which will set it to true 50% of the time
- Now try hitting `curl http://localhost:8080/mapped?name=somebody` with a couple of different names. Roughly 50% of them should return hello, the other half goodbye
- The response should be consistent for each name (eg "bob" will always receive the same greeting, "fred" will always receive the same greeting)
