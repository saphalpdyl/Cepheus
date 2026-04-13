FROM rust:slim-trixie

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    git

WORKDIR /app
    
RUN git clone https://github.com/asmie/stamp-suite.git .
RUN cargo build --release