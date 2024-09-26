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

Edit user_add.go to supply the authentication values (user and pass, or apiKey in the main() function.)

go get github.com/gorilla/websocket
go build user_add.go

./user_add <TrueNAS IP or server name>

It will attempt to log in, ping the server using the API ping call, and then create a user named "user2".  Failures will appear on stderr.



## Helpful Links

<a href="https://truenas.com">
<img align="right" src="https://www.truenas.com/docs/images/TrueNAS_Open_Enterprise_Storage.png" />
</a>

- [Websocket API docs](https://www.truenas.com/docs/api/scale_websocket_api.html)
- [Middleware repo](https://github.com/truenas/middleware)
- [Official TrueNAS Documentation Hub](https://www.truenas.com/docs/)
- [Get started building TrueNAS Scale](https://github.com/truenas/scale-build)
- [Forums](https://www.truenas.com/community/)
