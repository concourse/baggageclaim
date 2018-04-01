FROM golang:1

RUN apt-get update && \
    apt-get -y install \
        file \
        liblzo2-dev \
        libblkid-dev \
        e2fslibs-dev \
        pkg-config \
        libz-dev \
        libzstd-dev && \
    cd /tmp && \
    curl -L https://www.kernel.org/pub/linux/kernel/people/kdave/btrfs-progs/btrfs-progs-v4.15.tar.gz | tar zxf - && \
    cd btrfs-progs-* && \
    ./configure --disable-documentation && \
    make && \
    make install
