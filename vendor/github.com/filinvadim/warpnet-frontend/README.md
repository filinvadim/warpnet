# WARPNET-FRONTEND
## Requirements
    npm version >= 9.2.0
    golang >= 1.23 (brew install go)

## How to run node (dev mode)
- commit and push your frontend changes (INCLUDING DIST FOLDER!)
  to master branch (it's okay);
- switch to backend repo and call command:
```bash 
    make get-frontend
```
- in backend repo run node:
```bash 
    go run cmd/node/member/main.go
```
