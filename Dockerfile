FROM golang:1.26.2@sha256:5f3787b7f902c07c7ec4f3aa91a301a3eda8133aa32661a3b3a3a86ab3a68a36 AS builder

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
