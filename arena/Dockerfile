# Build context: arena/ (docker build -t arena .)
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS server
WORKDIR /src/engine
COPY engine/go.mod engine/go.sum ./
RUN go mod download
COPY engine/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/arena ./cmd/arena \
 && mkdir /out/data && chown 65532:65532 /out/data

FROM scratch
COPY --from=server /out/arena /arena
COPY --from=web /src/web/dist /web
COPY --from=server --chown=65532:65532 /out/data /data
USER 65532
EXPOSE 8080
VOLUME /data
ENTRYPOINT ["/arena", "-web", "/web", "-data", "/data"]
CMD ["-addr", ":8080"]
