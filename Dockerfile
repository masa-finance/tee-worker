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

RUN make build

RUN --mount=type=secret,id=private_key,dst=/app/tee/private.pem make sign

RUN make bundle

# Use the official Ubuntu 22.04 image as a base for the final image
FROM ${baseimage} AS base

COPY --from=builder /app/bin/masa-tee-worker /usr/bin/masa-tee-worker

# Create the 'masa' user and set up the home directory
RUN useradd -m -s /bin/bash masa && mkdir -p /home/masa && chown -R masa:masa /home/masa

# Switch to user 'masa' for following commands
USER masa

WORKDIR /home/masa

# Expose necessary ports
EXPOSE 8080

# Set default command to start the Go application
CMD ego run /usr/bin/masa-tee-worker