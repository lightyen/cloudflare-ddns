FROM gcr.io/distroless/base-nossl
COPY app /
WORKDIR /wd
CMD ["/app"]
