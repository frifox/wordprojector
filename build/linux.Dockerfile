# The image the Linux programs are built in (see build/dist.sh): Ubuntu 22.04 like the release workflow, because a program
# built against a newer glibc would not start on older systems. It is made for each
# architecture (docker build --platform ...): the Go build there is native for that architecture. Only what the build needs
# is installed, without recommended extras.
FROM ubuntu:22.04
ARG GOVER
ARG GOTAR=linux-amd64
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update -qq \
 && apt-get install -y -qq --no-install-recommends build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev curl ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && curl -fsSL "https://go.dev/dl/${GOVER}.${GOTAR}.tar.gz" | tar -C /usr/local -xz
ENV PATH=/usr/local/go/bin:$PATH GOFLAGS=-buildvcs=false
