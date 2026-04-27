#!/bin/sh
# Wait for NATS to be ready
until nats server check nats://nats:4222 2>/dev/null; do
  echo "Waiting for NATS..."
  sleep 1
done

# Create JetStream streams
nats stream add --server nats://nats:4222 METER_READINGS \
  --subjects "meter.*.readings" \
  --storage file \
  --retention limits \
  --max-age 1h \
  --max-msgs 1000000 \
  --discard old 2>/dev/null || true

nats stream add --server nats://nats:4222 EDGE_AGGREGATED \
  --subjects "edge.aggregated" \
  --storage file \
  --retention limits \
  --max-age 24h \
  --discard old 2>/dev/null || true

echo "NATS streams ready"