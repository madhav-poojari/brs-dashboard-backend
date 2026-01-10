# build stage
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -o /server ./cmd/dashboard-api

# runtime stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=build /server /server
EXPOSE 8080
ENTRYPOINT ["/server"]


