# Maand

Maand is a workload orchestrator and provisioner that operates without agents, with all states stored in a file.
It is designed to handle a wide variety of workloads within a cluster, automating the execution and management of jobs.

docs : https://maand.sh/latest

# How to build

``` 
export CGO_ENABLED=1 # maand uses sqlite3 for storage which needs cgo enabled.
go build
```