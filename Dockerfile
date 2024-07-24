FROM docker.io/golang:1.22-alpine3.20 AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /src

# Install project dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy project code
COPY . .

RUN go build -ldflags="-s -w" -o /build/radix-job-scheduler

# Final stage, ref https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md for distroless
FROM gcr.io/distroless/static
WORKDIR /app
COPY --from=builder /build/radix-job-scheduler .

USER 1000
ENTRYPOINT ["/app/radix-job-scheduler"]
