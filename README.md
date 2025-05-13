# WARPNET
[![Go Version](https://img.shields.io/badge/Go-1.24+-brightgreen)](https://golang.org/dl/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](./LICENSE)
[![Release](https://github.com/filinvadim/warpnet/actions/workflows/release.yaml/badge.svg)](https://github.com/filinvadim/warpnet/actions/workflows/release.yaml)
[![Telegram Chat](https://img.shields.io/badge/chat-telegram-blue.svg)](https://t.me/warpnetdev)

## General Principles of the Warp Network

1. WarpNet must operate independently of any third-party services.
2. WarpNet must not rely on any proprietary or third-party technologies.
3. A WarpNet node must be distributed as a single executable file.
4. WarpNet must be a cross-platform solution.
5. Only one WarpNet member node may run on a single machine, but it may have multiple aliases.
6. WarpNet member nodes must be governed solely by network consensus.
7. WarpNet business nodes may allow centralized management.
8. A WarpNet node must store private data only on the local host machine.
9. WarpNet member nodes must not be developed or controlled by a single individual.
10. Content on WarpNet must be moderated automatically, without human intervention.
11. Hosting a WarpNet bootstrap node must be incentivized with rewards.
12. Node owners bear full personal responsibility for any content they upload to WarpNet.


## Requirements
    golang >=1.24

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

Example:
```bash 
    go run cmd/node/member/main.go --database.dir storage2 --node.port 4021 --server.port 4022
```

## How to run multiple nodes in isolated network (dev mode)
1. In addition to the previous chapter update flags:
    - change `node.network.prefix` flag to different one.
2. Run multiple nodes as described in the previous chapter.

Example:
```bash 
    go run cmd/node/member/main.go --node.network.prefix myprefix
```

## Developers group in Telegram

https://t.me/warpnetdev

## License

Warpnet is free software licensed under the GNU General Public License v3.0 or later.

See the [LICENSE](./LICENSE) file for details.