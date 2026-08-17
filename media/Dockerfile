FROM golang:1.26.5-alpine AS build

WORKDIR /src
COPY go.work go.work.sum ./
COPY api/go.mod api/go.sum ./api/
COPY media/go.mod media/go.sum ./media/
COPY shared/go.mod ./shared/

COPY media ./media
COPY shared ./shared
RUN CGO_ENABLED=0 go build -o /out/media ./media/cmd/cli

FROM bluenviron/mediamtx:1 AS mediamtx

FROM alpine:3.22

RUN apk add --no-cache curl ffmpeg
COPY --from=mediamtx /mediamtx /mediamtx
COPY --from=build /out/media /usr/local/bin/media

ENTRYPOINT ["/mediamtx"]
