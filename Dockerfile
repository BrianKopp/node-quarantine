# Build stage
FROM golang:alpine as build

# Install git dep
RUN apk update
RUN apk add git

WORKDIR /go/src/github.com/briankopp/node-quarantine

# Copy over mod & sum files for dependency install
COPY go.* ./

# Install deps
RUN go mod download

# Copy over rest of code and build
COPY . .
WORKDIR /go/src/github.com/briankopp/node-quarantine/cmd/quarantine
RUN go build .
RUN ls -al

# Deploy stage
FROM alpine

EXPOSE 8080
COPY --from=build /go/src/github.com/briankopp/node-quarantine/cmd/quarantine/quarantine /app
ENTRYPOINT ["/app"]
