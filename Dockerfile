# Use the official Golang image as the parent image
FROM golang:alpine AS builder

RUN apk add --no-cache ca-certificates git
# Set the working directory to /go/src/app
WORKDIR /go/src/app

# Copy the current directory contents into the container at /go/src/app
COPY . /go/src/app

# Install any needed packages
RUN go mod download

# Build the static binary
# CGO_ENABLED=0 ensures a statically linked binary, making it portable to the final scratch image
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/todo-server ./examples/todo-server

FROM scratch

COPY --from=builder /go/src/app/bin/todo-server ./
# Expose port 8080 for the application
EXPOSE 8099

# Run the http-service
ENTRYPOINT ["/todo-server"]