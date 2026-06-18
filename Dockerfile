# Minimal runtime image; goreleaser injects the prebuilt static binary.
FROM gcr.io/distroless/static:nonroot
COPY opusclip /usr/local/bin/opusclip
ENTRYPOINT ["/usr/local/bin/opusclip"]
