FROM ubuntu:16.04

RUN apt update                    && \
    apt install -y gcc g++ distcc && \
    apt clean                     && \
    rm -rf /var/cache/apt

RUN useradd distcc
CMD ["distccd"]
