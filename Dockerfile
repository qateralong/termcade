# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/termcade ./cmd/termcade \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/termcade /termcade
COPY --from=build --chown=nonroot:nonroot /out/data /data
ENV TERMCADE_ADDR=:2222 \
    TERMCADE_HOST_KEY=/data/host_ed25519 \
    TERMCADE_DB=/data/termcade.db
VOLUME /data
EXPOSE 2222
ENTRYPOINT ["/termcade"]
