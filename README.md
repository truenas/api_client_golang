<p align="center">
      <a href="https://discord.gg/Q3St5fPETd"><img alt="Join Discord" src="https://badgen.net/discord/members/Q3St5fPETd/?icon=discord&label=Join%20the%20TrueNAS%20Community" /></a>
 <a href="https://www.truenas.com/community/"><img alt="Join Forums" src="https://badgen.net/badge/Forums/Post%20Now//purple" /></a>
 <a href="https://jira.ixsystems.com"><img alt="File Issue" src="https://badgen.net/badge/Jira/File%20Issue//red?icon=jira" /></a>
</p>

# TrueNAS Go Websocket Client


## About

This is a an example for using the TrueNAS Websocket API using Go.  It parallels the Python TrueNAS API client, found here:
https://github.com/truenas/api_client

It is JSON-RPC 2.0 based, so it doesn't use the ws://truenas.address/websocket API.

## Getting Started

On any system with Go installed  (see https://go.dev/doc/install), clone this repo.

Then build the truenas_go command:
```
go build truenas_go.go
go install truenas_go.go
```

Example run:
```
export TRUENAS_API_KEY="1-xxxxxxxxx" # Create this on your TrueNAS, and copy the result here
truenas_go --uri ws://ip_of_your_truenas/api/current --api-key=${TRUENAS_API_KEY} --timeout 20 --method system.info
```

More command line examples in EXAMPLES.md

Code examples in `examples/`

See `examples/app_upgrade/app_upgrade.go` for an example that uses a long running job and progress reporting.



## Helpful Links

<a href="https://truenas.com">
<img align="right" src="https://www.truenas.com/docs/images/TrueNAS_Open_Enterprise_Storage.png" />
</a>

- [Websocket API docs](https://www.truenas.com/docs/api/scale_websocket_api.html)
- [Middleware repo](https://github.com/truenas/middleware)
- [Official TrueNAS Documentation Hub](https://www.truenas.com/docs/)
- [Get started building TrueNAS Scale](https://github.com/truenas/scale-build)
- [Forums](https://www.truenas.com/community/)
