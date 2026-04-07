FROM gcr.io/distroless/static-debian12
ARG TARGETARCH
COPY --chmod=0755 ./postbox-linux-${TARGETARCH} /usr/bin/postbox
ENV XDG_CONFIG_HOME=/var/lib
ENV XDG_DATA_HOME=/var/lib
ENTRYPOINT ["/usr/bin/postbox"]
CMD ["server"]
