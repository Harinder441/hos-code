# Stage 1: Bundle frontend assets
FROM node:20-alpine AS web-builder
WORKDIR /build
COPY scripts/ ./scripts/
COPY web/ ./web/
RUN node ./scripts/build-web.js

# Stage 2: Build static Go binary
FROM golang:1.25-alpine AS go-builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy generated bundle into web/
COPY --from=web-builder /build/web/app.js ./web/app.js
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/hos-code .

# Stage 3: Minimal runtime container
FROM alpine:3.21
RUN apk --no-cache add ca-certificates git
COPY --from=go-builder /bin/hos-code /usr/local/bin/hos-code
COPY LICENSE THIRD_PARTY_NOTICES.md /usr/share/doc/hos-code/

EXPOSE 7777
ENTRYPOINT ["hos-code", "-host", "0.0.0.0", "-no-open"]
CMD ["/workspace"]
