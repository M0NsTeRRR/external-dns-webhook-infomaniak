FROM golang:1.26.1@sha256:c7e98cc0fd4dfb71ee7465fee6c9a5f079163307e4bf141b336bb9dae00159a5 AS builder

ARG VERSION=development
ARG SOURCE_DATE_EPOCH=0

WORKDIR /go/src/app

COPY . .

RUN go mod download

RUN CGO_ENABLED=0 go build -trimpath -a -o external-dns-webhook-infomaniak -ldflags '-w -X main.version=$VERSION -X main.buildTime=$SOURCE_DATE_EPOCH -extldflags "-static"' cmd/webhook/main.go

FROM gcr.io/distroless/static:nonroot@sha256:e3f945647ffb95b5839c07038d64f9811adf17308b9121d8a2b87b6a22a80a39

COPY --from=builder /go/src/app/external-dns-webhook-infomaniak /bin/external-dns-webhook-infomaniak

USER 65532

ENTRYPOINT ["/bin/external-dns-webhook-infomaniak"]
