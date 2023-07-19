FROM golang:1.20-alpine3.18 as builder
ENV GO111MODULE=on

RUN addgroup -S -g 1000 job-scheduler
RUN adduser -S -u 1000 -G job-scheduler job-scheduler

RUN apk update && apk upgrade && \
    apk add bash jq alpine-sdk sed gawk git ca-certificates curl && \
    apk add --no-cache gcc musl-dev
RUN go install honnef.co/go/tools/cmd/staticcheck@v0.3.3 && \
    go install github.com/rakyll/statik@v0.1.7

    # Install go-swagger - 57786786=v0.29.0 - get release id from https://api.github.com/repos/go-swagger/go-swagger/releases
RUN download_url=$(curl -s https://api.github.com/repos/go-swagger/go-swagger/releases/57786786 | \
    jq -r '.assets[] | select(.name | contains("'"$(uname | tr '[:upper:]' '[:lower:]')"'_amd64")) | .browser_download_url') && \
    curl -o /usr/local/bin/swagger -L'#' "$download_url" && \
    chmod +x /usr/local/bin/swagger

WORKDIR /go/src/github.com/equinor/radix-job-scheduler/

# get dependencies
COPY go.mod go.sum ./
RUN go mod download

# copy api code
COPY . .

# Generate swagger
RUN swagger generate spec -o ./swaggerui_src/swagger.json --scan-models -x github.com/equinor/radix-job-scheduler/models/v2 && \
    swagger validate ./swaggerui_src/swagger.json && \
    statik -src=./swaggerui_src/ -p swaggerui

# lint and unit tests
RUN staticcheck ./... && \
    go vet ./... && \
    CGO_ENABLED=0 GOOS=linux go test ./...

# Build radix api go project
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o /usr/local/bin/radix-job-scheduler

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /usr/local/bin/radix-job-scheduler /usr/local/bin/radix-job-scheduler

EXPOSE 8080
USER 1000
ENTRYPOINT ["/usr/local/bin/radix-job-scheduler"]