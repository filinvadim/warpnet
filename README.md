# WARPNET
## ! General principles !
- warpnet must be independent of any third party services
- warpnet must be independent of any third party technologies
- warpnet node must be represented as a single executable file
- warpnet must be a multiplatform service
- warpnet must be a singleton node that running on a machine, but it could have as many aliases as needed
- warpnet member node must be managed only by node consensus
- warpnet business node could have centralized management
- warpnet node must store private data only at host machine
- warpnet member node must not be developed by a single programmer
- warpnet contents must be moderated without any human being intervention
- warpnet bootstrap node hosting must be rewarded

## 🛠 MVP Features supported (2025-04-25)

| **Category**       | **Feature**                     | **Description**                                       | **Completed** |
|--------------------|---------------------------------|-------------------------------------------------------|---------------|
| **Authentication** | Node registration               | Key pair generation, initial setup                    | ✅             |
|                    | Public key exchange             | RSA/Ed25519 key sharing with peers                    | ✅             |
|                    | Challenge-Response              | Identity verification protocol between nodes          | ✅             |
| **User**           | Create profile                  | Nickname, bio, public key                             | ✅             |
|                    | Fetch profile                   | By ID or public key                                   | ✅             |
| **Social Graph**   | Subscribe to followee           | Add to following list (follower → followee)           | ✅             |
|                    | Unsubscribe                     | Remove a followee from the list                       | ✅             |
| **Tweets**         | Publish tweet                   | Text content with local timestamp                     | ✅             |
|                    | Fetch own tweets                | From local BadgerDB                                   | ✅             |
|                    | Fetch tweets from followees     | Pull or receive via push, verify signature            | ✅             |
|                    | Upload 1 picture with tweet     | Add media to tweets                                   |               |
| **Timeline**       | Merge timeline                  | Aggregate tweets from followed users                  | ✅             |
|                    | Pagination / limit              | Limit or paginate timeline results                    | ✅             |
| **Networking**     | Node discovery & connection     | p2p discovery + manual friend list                    | ✅             |
|                    | API between nodes               | Exchange tweets, subs, profiles                       | ✅             |
|                    | Broadcast support               | Push tweets to known friends                          | ✅             |
| **Security**       | Sign tweets                     | ECDSA/RSA signatures on content                       |               |
|                    | Verify incoming tweets          | Check signature validity                              |               |
|                    | Rate limiting / IP filtering    | Basic DoS/DDoS protection                             | ✅             |
| **Moderation**     | Content filtering (via AI node) | Enforce basic human rights (CP, weapons, fraud, etc.) |               |
| **Monitoring**     | Prometheus metrics              | Track request counts, errors, etc.                    |               |

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
