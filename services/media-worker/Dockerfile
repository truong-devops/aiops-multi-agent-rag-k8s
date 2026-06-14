FROM golang:1.24.13-alpine3.23 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ffmpeg ca-certificates
WORKDIR /app
COPY --from=build /out/server /app/server
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/app/server"]
