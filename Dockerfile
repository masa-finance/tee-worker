FROM rust:1.71.1 as builder

WORKDIR /app

COPY . .

RUN cargo build --release

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/target/release/tee-worker /app/

EXPOSE 8080

CMD ["./tee-worker"]

# Updated PCCS server URL
ENV PCCS_URL=https://pccs.masa.ai