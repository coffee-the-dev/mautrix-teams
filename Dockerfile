FROM docker.io/library/golang:1.26-alpine AS build

RUN apk add --no-cache git gcc musl-dev olm-dev
WORKDIR /src
COPY . .
ARG TARGETARCH
RUN CGO_ENABLED=1 GOOS=linux GOARCH=$TARGETARCH \
    MAUTRIX_VERSION="$(grep 'maunium.net/go/mautrix ' go.mod | awk '{print $2}' | head -n1)" \
    go build -trimpath \
    -ldflags="-s -w -X main.Tag=$(git describe --tags --always) -X main.Commit=$(git rev-parse HEAD) -X 'maunium.net/go/mautrix.GoModVersion=${MAUTRIX_VERSION}'" \
    -o /out/mautrix-teams ./cmd/mautrix-teams

FROM docker.io/library/alpine:3.22

RUN apk add --no-cache ffmpeg olm ca-certificates su-exec
COPY --from=build /out/mautrix-teams /usr/bin/mautrix-teams
ENV UID=1337 GID=1337 BRIDGEV2=1
VOLUME /data
WORKDIR /data
ENTRYPOINT ["/usr/bin/mautrix-teams"]
CMD ["-c", "/data/config.yaml", "--no-update"]
