FROM bluenviron/mediamtx:1 AS mediamtx

FROM alpine:3.22

RUN apk add --no-cache curl ffmpeg
COPY --from=mediamtx /mediamtx /mediamtx

ENTRYPOINT ["/mediamtx"]
