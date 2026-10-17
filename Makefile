test:
	@go test ./... -cover

build-todo:
	@go build -o bin/todo-server ./examples/todo-server

run-todo: build-todo
	@./bin/todo-server

docker:
	docker build -t featurehub/golang-sdk-todo .

docker-run: docker
	docker run -e FEATUREHUB_CLIENT_API_KEY -e FEATUREHUB_EDGE_URL -p 8099:8099 featurehub/golang-sdk-todo

