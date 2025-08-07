ARG egover=1.7.2
ARG baseimage=ghcr.io/edgelesssys/ego-deploy:v${egover}
ARG VERSION

# Build the Go binary in a separate stage utilizing Makefile
FROM ghcr.io/edgelesssys/ego-dev:v${egover} AS dependencies

WORKDIR /app
# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download
# Copy the rest of the source
COPY . .

# Build the Go binary in a separate stage utilizing Makefile
FROM dependencies AS builder

ARG VERSION
ENV VERSION=${VERSION}
ARG DISTRIBUTOR_PUBKEY
ARG MINERS_WHITE_LIST
ENV MINERS_WHITE_LIST=${MINERS_WHITE_LIST}
ENV DISTRIBUTOR_PUBKEY=${DISTRIBUTOR_PUBKEY}
RUN make build


RUN --mount=type=secret,id=private_key,dst=/app/tee/private.pem make ci-sign

RUN make bundle

# Use the official Ubuntu 22.04 image as a base for the final image
FROM ${baseimage} AS base
ARG pccs_server=https://pccs.masa.ai

# Install Intel SGX DCAP driver
RUN apt-get update && \
    apt-get install -y lsb-core && \
    mkdir -p /etc/apt/keyrings && \
    wget -qO- https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | tee /etc/apt/keyrings/intel-sgx-keyring.asc > /dev/null && \
    echo "deb [signed-by=/etc/apt/keyrings/intel-sgx-keyring.asc arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/intel-sgx.list && \
    apt-get update && \
    apt-get install -y libsgx-dcap-default-qpl
RUN sed -i 's#"pccs_url": *"[^"]*"#"pccs_url": "'${pccs_server}'/sgx/certification/v4/"#' /etc/sgx_default_qcnl.conf
RUN sed -i 's#"use_secure_cert": true#"use_secure_cert": false#' /etc/sgx_default_qcnl.conf

COPY --from=builder /app/bin/masa-tee-worker /usr/bin/masa-tee-worker

# Create the 'masa' user and set up the home directory
RUN useradd -m -s /bin/bash masa && mkdir -p /home/masa && chown -R masa:masa /home/masa

WORKDIR /home/masa
ENV DATA_DIR=/home/masa

# Expose necessary ports
EXPOSE 8080

# Set default command to start the Go application
CMD ["ego", "run", "/usr/bin/masa-tee-worker"]
