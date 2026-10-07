FROM golang:1.27-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /event-generator .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /event-generator /event-generator
ENTRYPOINT ["/event-generator"]
