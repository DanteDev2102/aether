#!/bin/sh

for i in $(seq 1 1000); do
    curl http://localhost:8080/items -X POST -H "Content-Type: application/json" -d ' {"name": "test", "qty": 10 }'
done

curl http://locahost:8080/session/profile
