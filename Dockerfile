ARG egover=1.6.0
ARG baseimage=ghcr.io/edgelesssys/ego-deploy:v${egover}
ARG VERSION

# Build the Go binary in a separate stage utilizing Makefile
FROM ghcr.io/edgelesssys/ego-dev:v${egover} AS dependencies

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build the Go binary in a separate stage utilizing Makefile
FROM dependencies AS builder

ENV VERSION=${VERSION}
ARG DISTRIBUTOR_PUBKEY
RUN DISTRIBUTOR_PUBKEY=${DISTRIBUTOR_PUBKEY} make build

RUN --mount=type=secret,id=private_key,dst=/app/tee/private.pem make sign

RUN make bundle

# Use the official Ubuntu 22.04 image as a base for the final image
FROM ${baseimage} AS base
ARG pccs_server=https://pccs.dev.masalabs.ai

# Install Intel SGX DCAP driver
RUN mkdir -p /etc/apt/keyrings && \
    wget -qO- https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | tee /etc/apt/keyrings/intel-sgx-keyring.asc > /dev/null && \
    /bin/bash -c 'echo "deb [signed-by=/etc/apt/keyrings/intel-sgx-keyring.asc arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu $(lsb_release -cs) main' | tee /etc/apt/sources.list.d/intel-sgx.list && \
    apt-get update && \
    apt-get install -y libsgx-dcap-default-qpl
RUN sed -i 's#"pccs_url": *"[^"]*"#"pccs_url": "'${pccs_server}'/sgx/certification/v4/"#' /etc/sgx_default_qcnl.conf

COPY --from=builder /app/bin/masa-tee-worker /usr/bin/masa-tee-worker

# Create the 'masa' user and set up the home directory
RUN useradd -m -s /bin/bash masa && mkdir -p /home/masa && chown -R masa:masa /home/masa

WORKDIR /home/masa
ENV DATA_DIR=/home/masa

# Expose necessary ports
EXPOSE 8080

# Set default command to start the Go application
CMD ego run /usr/bin/masa-tee-worker
