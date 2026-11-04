FROM golang:1.27.3@sha256:2920bb124f1a2e12a1d39f0cab756ae0068441ee1da7655dec7f5e374944095b AS builder

ARG VERSION=development
ARG SOURCE_DATE_EPOCH=0

WORKDIR /go/src/app

COPY . .

RUN go mod download

RUN CGO_ENABLED=0 go build -trimpath -a -o external-dns-webhook-infomaniak -ldflags "-w -X main.version=$VERSION -X main.buildTime=$SOURCE_DATE_EPOCH -extldflags '-static'" cmd/webhook/main.go

FROM gcr.io/distroless/static:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

COPY --from=builder /go/src/app/external-dns-webhook-infomaniak /bin/external-dns-webhook-infomaniak

USER 65532

ENTRYPOINT ["/bin/external-dns-webhook-infomaniak"]
