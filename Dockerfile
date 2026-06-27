FROM golang:1.26.4@sha256:32c0e6e5c4f6707717051091b4d0b077464a679eaab563e11474efc5328e2aa5 AS builder

ARG VERSION=development
ARG SOURCE_DATE_EPOCH=0

WORKDIR /go/src/app

COPY . .

RUN go mod download

RUN CGO_ENABLED=0 go build -trimpath -a -o external-dns-webhook-infomaniak -ldflags "-w -X main.version=$VERSION -X main.buildTime=$SOURCE_DATE_EPOCH -extldflags '-static'" cmd/webhook/main.go

FROM gcr.io/distroless/static:nonroot@sha256:963fa6c544fe5ce420f1f54fb88b6fb01479f054c8056d0f74cc2c6000df5240

COPY --from=builder /go/src/app/external-dns-webhook-infomaniak /bin/external-dns-webhook-infomaniak

USER 65532

ENTRYPOINT ["/bin/external-dns-webhook-infomaniak"]
