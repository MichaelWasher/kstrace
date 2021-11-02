FROM docker.io/alpine:latest

# Install Crictl and strace
RUN VERSION="v1.22.0" && \
    wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-amd64.tar.gz && \
    tar zxvf crictl-$VERSION-linux-amd64.tar.gz -C /bin  && \
    rm -f crictl-$VERSION-linux-amd64.tar.gz && \
    apk add --no-cache strace coreutils 

# Set the default Endpoint for CRI-O 
ENV CONTAINER_RUNTIME_ENDPOINT=unix:///run/crio/crio.sock
RUN mkdir -p /run/crio/ && touch /run/crio/crio.sock

