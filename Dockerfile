FROM golang:1.21-alpine3 as builder
ENV GO111MODULE=on

RUN addgroup -S -g 1000 job-scheduler
RUN adduser -S -u 1000 -G job-scheduler job-scheduler

RUN apk update && apk upgrade && \
    apk add bash jq alpine-sdk sed gawk git ca-certificates curl && \
    apk add --no-cache gcc musl-dev

WORKDIR /go/src/github.com/equinor/radix-job-scheduler/

# get dependencies
COPY go.mod go.sum ./
RUN go mod download

# copy api code
COPY . .

# Build radix api go project
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -a -installsuffix cgo -o /usr/local/bin/radix-job-scheduler

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /usr/local/bin/radix-job-scheduler /usr/local/bin/radix-job-scheduler

EXPOSE 8080
USER 1000
ENTRYPOINT ["/usr/local/bin/radix-job-scheduler"]
