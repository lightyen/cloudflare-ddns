FROM gcr.io/distroless/base-nossl
COPY app /
WORKDIR /data
CMD ["/app"]
