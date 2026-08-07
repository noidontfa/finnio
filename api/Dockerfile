FROM golang:1.23.5-alpine AS build

WORKDIR /src
COPY go.work go.work.sum ./
COPY api/go.mod api/go.sum ./api/
COPY shared/go.mod ./shared/
RUN cd api && go mod download

COPY api ./api
COPY shared ./shared
RUN CGO_ENABLED=0 go build -o /out/api ./api/cmd/server

FROM alpine:3.22

RUN addgroup -S finnio && adduser -S finnio -G finnio
WORKDIR /app
COPY --from=build /out/api /usr/local/bin/api
COPY --chown=finnio:finnio api/data ./data

USER finnio
EXPOSE 5555
ENTRYPOINT ["api"]
