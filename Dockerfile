# Use the official Golang image as the parent image
FROM golang:latest

# Set the working directory to /go/src/app
WORKDIR /go/src/app

# Copy the current directory contents into the container at /go/src/app
COPY . /go/src/app

# Install any needed packages
RUN go get -d -v ./...

# Expose port 8080 for the application
EXPOSE 8080

# Change working directory.
WORKDIR /go/src/app/examples/http-service

# Run the http-service
ENTRYPOINT ["go","run","main.go"]