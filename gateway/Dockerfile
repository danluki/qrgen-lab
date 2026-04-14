FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY gateway ./gateway
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gateway ./gateway

FROM alpine:3.20
RUN adduser -D -H appuser
USER appuser
COPY --from=build /out/gateway /usr/local/bin/gateway
EXPOSE 8080
CMD ["/usr/local/bin/gateway"]
