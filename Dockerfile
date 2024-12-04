FROM golang:1.23 as builder
WORKDIR /build
# Copy your manifest and any other necessary files
COPY manifest.yaml ./
COPY processor/ ./processor/
# Install the OpenTelemetry Collector Builder
RUN go install go.opentelemetry.io/collector/cmd/builder@v0.114.0
# Build the collector for Linux and verify the output
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 builder --config manifest.yaml && \
    ls -la

FROM alpine:3.20 AS certs
RUN apk --update add ca-certificates

FROM scratch
ARG USER_UID=10001
USER ${USER_UID}
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
# The builder creates the binary in ./_build/otelcol-contrib by default
COPY --from=builder --chmod=755 /build/_build/otelcol-contrib /otelcol-contrib
COPY config.yaml /etc/otelcol-contrib/config.yaml
ENTRYPOINT ["/otelcol-contrib"]
CMD ["--config", "/etc/otelcol-contrib/config.yaml"]
EXPOSE 4317 4318 55678 55679
