FROM golang:1.6

RUN apt-get update && \
    apt-get -y install \
        file \
        liblzo2-dev \
        libblkid-dev \
        e2fslibs-dev \
        pkg-config \
        libz-dev && \
    cd /tmp && \
    curl https://www.kernel.org/pub/linux/kernel/people/kdave/btrfs-progs/btrfs-progs-v4.4.tar.gz | tar zxf - && \
    cd btrfs-progs-v4.4 && \
    ./configure --disable-documentation && \
    make && \
    make install

