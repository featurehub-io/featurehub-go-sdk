test:
	@go test ./... -cover

build-todo:
	@go build -o bin/todo-server ./examples/todo-server

run-todo: build-todo
	@./bin/todo-server
