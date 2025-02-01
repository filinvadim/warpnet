# WARPNET
## General principles
- warpnet must be independent of any 3rd party services
- warpnet must be independent of any 3rd party technologies
- warpnet node must be represented as a single executable file
- warpnet must be a multiplatform service
- warpnet must be a singleton node that running on machine, but it could have as many aliases as needed
- warpnet member node must be managed only by nodes consensus
- warpnet business node could have centralized management
- warpnet node must store private data only at host machine
- warpnet member node must not be developed by a single programmer
## Requirements
    golang >=1.23

## How to run
- bootstrap node
```bash 
go run cmd/node/bootstrap/main.go
```
- member node
```bash 
go run cmd/node/member/main.go
```

## Warpnet API
[HTTP REST API specification](spec/local-api.yml)

[WS event API specification](spec/event-api.yml)
