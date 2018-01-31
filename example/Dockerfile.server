FROM ubuntu:16.04

RUN apt update                    && \
    apt install -y gcc g++ distcc && \
    apt clean                     && \
    rm -rf /var/cache/apt

# Add Tini
ENV TINI_VERSION v0.16.1
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini /tini
RUN chmod +x /tini
ENTRYPOINT ["/tini", "--"]

RUN useradd distcc
CMD ["distccd"]
