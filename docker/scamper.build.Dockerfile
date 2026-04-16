ARG SCAMPER_VERSION=20260331

FROM dockcross/linux-x64:latest AS builder
ARG SCAMPER_VERSION

WORKDIR /src

RUN wget https://www.caida.org/catalog/software/scamper/code/scamper-cvs-${SCAMPER_VERSION}.tar.gz
RUN tar -vxzf scamper-cvs-${SCAMPER_VERSION}.tar.gz
WORKDIR /src/scamper-cvs-${SCAMPER_VERSION}

RUN LDFLAGS="-static -Wl,--allow-multiple-definition" \
    ./configure --host=${CROSS_TRIPLE} \
    --enable-static --disable-shared \
    --disable-scamper-tbit --disable-scamper-sting \
    --disable-scamper-sniff --disable-scamper-dealias \
    --disable-scamper-host --disable-scamper-http

RUN make LDFLAGS="-all-static -Wl,--allow-multiple-definition"

FROM scratch
ARG SCAMPER_VERSION
COPY --from=builder /src/scamper-cvs-${SCAMPER_VERSION}/scamper/scamper /cepheus-agent/scamper
