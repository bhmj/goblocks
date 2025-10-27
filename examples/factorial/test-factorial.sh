#!/bin/bash

echo "--- Building application ---"
go build -o factorial factorial.go
echo "--- Starting application ---"
./factorial --config-file=config.yaml &
PID=$!

echo "--- Waiting for ready signal ---"
until curl -fs -o /dev/null 127.0.0.1:8081/ready; do
  sleep 1
done

echo "--- Request to API ---"
curl -v http://localhost:8080/api/v1/factorial/55
echo

echo "--- Stopping application ---"
kill -TERM "$PID"
wait "$PID" || true
