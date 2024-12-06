# Trading App

- `superset/`: exported data (dashboards, charts, etc.) from Superset
- `k8s/`: Kubernetes config. Uses Helm via a central Helmfile.
- `data/`: CSV files from dataset downloaded from external source
- `shared/`: shared generated Go datatypes
- `producer/`: Go program for producing data from the datasets
- `trade-eventer/`: Go program that creates buy/sell events

Main database schema: `k8s/clickhouse/init.sql`. Protocol Buffer schemas are also in `k8s/clickhouse/`. Protocol buffers can be generated into Go variants using `sh generate-protos.sh`.

Build `trade-eventer` Docker image using `sh build.sh`.

The trade event producer (`producer`) is used as needed, and thus is run outside of the Kubernetes cluster using `go run .` in the directory.

For deploying system components, look at `k8s/README.md`.
