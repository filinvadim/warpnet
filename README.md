# WARPNET
## General principles
- warpnet must be independent of any third party services
- warpnet must be independent of any third party technologies
- warpnet node must be represented as a single executable file
- warpnet must be a multiplatform service
- warpnet must be a singleton node that running on machine, but it could have as many aliases as needed
- warpnet member node must be managed only by node consensus
- warpnet business node could have centralized management
- warpnet node must store private data only at host machine
- warpnet member node must not be developed by a single programmer
## Requirements
    golang >=1.23

## How to run single node (dev mode)
- bootstrap node
```bash 
    go run cmd/node/bootstrap/main.go
```
- member node
```bash 
    go run cmd/node/member/main.go
```

## How to run multiple nodes (dev mode)
1. go to `config/config.go`:
   - change all ports to different ones;
   - change `database.dir` flag to different one.
2. Run every node as an independent OS process
   as described in the previous chapter.

## How to run multiple nodes in isolated network (dev mode)
1. In addition to the previous chapter update flags:
    - change `node.network.prefix` flag to different one.
2. Run multiple nodes as described in the previous chapter.

