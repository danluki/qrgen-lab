FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY internal ./internal
COPY qrgen ./qrgen
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/qrgen ./qrgen

FROM alpine:3.20
RUN adduser -D -H appuser
USER appuser
COPY --from=build /out/qrgen /usr/local/bin/qrgen
CMD ["/usr/local/bin/qrgen"]
