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

## How to run single node (dev mode)
- create stub binary signing keys
```bash 
cp warpnet.example.sig warpnet.sig 
cp public.example.pem public.pem
```
- bootstrap node
```bash 
go run cmd/node/bootstrap/main.go
```
- member node
```bash 
go run cmd/node/member/main.go
```

## How to run multiple nodes (dev mode)
1. Update [config](./config.yml) file:
   - change all ports to different ones;
   - change `database/dirName` to different one.
2. Run every node as independent OS process
as described in previous chapter.

## How to run multiple nodes in isolated network (dev mode)
1. In addition to previous chapter update [config](./config.yml) file:
    - change `node/network_prefix` to different one.
2. Run multiple nodes as described in previous chapter.


## Warpnet API
[HTTP REST API specification](spec/local-api.yml)

