FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o dsl-engine .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/dsl-engine /dsl-engine
COPY default.rules /rules/default.rules
ENTRYPOINT ["/dsl-engine"]