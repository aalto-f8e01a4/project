protoc -I=. --go_out=./shared --go_opt=Mk8s/clickhouse/trade.proto=. ./k8s/clickhouse/trade.proto
protoc -I=. --go_out=./shared --go_opt=Mk8s/clickhouse/action.proto=. ./k8s/clickhouse/action.proto