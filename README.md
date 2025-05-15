[![Go Version](https://img.shields.io/badge/Go-1.24+-brightgreen)](https://golang.org/dl/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](./LICENSE)
[![Release](https://github.com/Warp-net/warpnet/actions/workflows/release.yaml/badge.svg)](https://github.com/Warp-net/warpnet/actions/workflows/release.yaml)
[![Telegram Chat](https://img.shields.io/badge/chat-telegram-blue.svg)](https://t.me/warpnetdev)
<br />
<div align="center">
  <a href="https://github.com/Warp-net/warpnet">
    <img src="docs/logo.png" alt="Logo" width="80" height="80">
  </a>

<h3 align="center">Warpnet</h3>

  <p align="center">
    An awesome README template to jumpstart your projects!
    <br />
    <a href="https://github.com/Warp-net/docs"><strong>Explore the docs »</strong></a>
    <br />
    <br />
    &middot;
    <a href="https://github.com/Warp-net/warpnet/issues/new?labels=bug&template=bug-report---.md">Report Bug</a>
    &middot;
    <a href="https://github.com/Warp-net/warpnet/issues/new?labels=enhancement&template=feature-request---.md">Request Feature</a>
  </p>
</div>

<summary>Table of Contents</summary>
<ol>
    <li>
      <a href="#general-principles-of-the-warp-network">General Principles</a>
    </li>
    <li>
      <a href="#getting-started">Getting Started</a>
      <ul>
        <li><a href="#prerequisites">Prerequisites</a></li>
        <li><a href="#usage-and-available-options">Usage and available options</a></li>
        <li><a href="#how-to-run-single-node-dev-mode">How to run single node (dev mode)</a></li>
        <li><a href="#how-to-run-multiple-nodes-dev-mode">How to run multiple nodes (dev mode)</a></li>
        <li><a href="#how-to-run-multiple-nodes-in-isolated-network-dev-mode">How to run multiple nodes in isolated network (dev mode)</a></li>
      </ul>
    </li>
    <li><a href="#TODO">TODO</a></li>
    <li><a href="#Contributing">Contributing</a></li>
    <li><a href="#Contact">Contact</a></li>
    <li><a href="#License">License</a></li>
</ol>

![Screenshot](docs/warpscreen.jpg)

## General Principles of the Warp Network

1. WarpNet cannot be owned by anyone.
2. WarpNet must operate independently of any third-party services.
3. WarpNet must not rely on any proprietary or third-party technologies.
4. A WarpNet node must be distributed as a single executable file.
5. WarpNet must be a cross-platform solution.
6. Only one WarpNet member node may run on a single machine, but it may have multiple aliases.
7. WarpNet member nodes must be governed solely by network consensus.
8. WarpNet business nodes may allow centralized management.
9. WarpNet business nodes must responsibly run at least one bootstrap node.
10. A WarpNet node must store private data only on the local host machine.
11. WarpNet member nodes must not be developed or controlled by a single individual.
12. Content on WarpNet must be moderated automatically, without human intervention.
13. Hosting a WarpNet bootstrap node must be incentivized with rewards.
14. Node owners bear full personal responsibility for any content they upload to WarpNet.

## Getting Started
### Prerequisites

List of software needed and how to install them.
* [Golang](https://go.dev/doc/install)

### Usage and available options

```bash 
    --database.dir string          Database directory name (default "storage")
    --logging.level string         Logging level (default "info")
    --node.bootstrap string        Bootstrap nodes multiaddr list, comma separated
    --node.host string             Node host (default "0.0.0.0")
    --node.inmemory                Bootstrap node runs without persistent storage
    --node.metrics.server string   Metrics push server address
    --node.network string          Private network. Use 'testnet' for testing env. (default "testnet")
    --node.port string             Node port (default "4001")
    --node.seed string             Bootstrap node seed for deterministic ID generation (random string)
    --server.host string           Server host (default "localhost")
    --server.port string           Server port (default "4002")
```
The above parameters also could be set as environment variables:
```
    NODE_PORT=4001
    NODE_SEED=warpnet1
    NODE_HOST=207.154.221.44
    LOGGING_LEVEL=debug 
    ...
```

### How to run single node (dev mode)
- bootstrap node
```bash 
    go run cmd/node/bootstrap/main.go
```
- member node
```bash 
    go run cmd/node/member/main.go
```

### How to run multiple nodes (dev mode)
Change database directory name and ports. Run every node as an independent OS process.
```bash 
    go run cmd/node/member/main.go --database.dir storage2 --node.port 4021 --server.port 4022
```

### How to run multiple nodes in isolated network (dev mode)
In addition to the previous chapter update flags - change `node.network` flag to different one.
```bash 
    go run cmd/node/member/main.go --node.network myownnetwork
```

## TODO
- [ ] Set up a website
  - [ ] Add docs to website
  - [ ] Run URL shortener server
- [ ] Create DMG package
- [ ] Create Snap package
- [ ] Set up bootstrap nodes on each continent
- [ ] TODO

See the [open issues](https://github.com/Warp-net/warpnet/issues) for a full list of proposed features (and known issues).

## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create.
Any contributions you make are **greatly appreciated**.

If you have a suggestion that would make this better, please fork the repo and create a pull request. 
You can also open an issue with the tag "enhancement."
Remember to give the project a star! Thanks again!

1. Fork the Project
2. Create your Feature Branch (`git checkout -b IssueNumber/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin IssueNumber/AmazingFeature`)
5. Open a Pull Request

### Top contributors:

<a href="https://github.com/Warp-net/warpnet/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Warp-net/warpnet" alt="contrib.rocks image" />
</a>

## Contact

### Maintainer 
* Vadim Filin - github.com.mecdy@passmail.net

### Developers group in Telegram

* [warpnetdev](https://t.me/warpnetdev)

## License

Warpnet is free software licensed under the GNU General Public License v3.0 or later.

See the [LICENSE](./LICENSE) file for details.