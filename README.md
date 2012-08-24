gosockjs
========

A Go language implementation of a [SockJS](https://github.com/sockjs/sockjs-client) server.
The protocol is partly specified in a [test suite](https://github.com/sockjs/sockjs-protocol/blob/master/sockjs-protocol-0.3.py). Read that for details.

Only the websocket, raw-websocket, xhr, and xhr_streaming, iframe_xhr(polling|streaming) protocols are supported.

UNDER CONSTRUCTION. It is not ready for use.

There is a test server in test_server that can be used with the sockjs-protocol suite (go run test_server/server.go). There is a simple client that acts as a quick smoke/sanity test in test_client.

Some obvious TODOs:
* Other protocols, maybe.
* Cookies.
* Bulletproof parallelism issues.
* Real testing. There are some tests here, but not nearly enough. The sockjs-protocol tests are not at all thorough.
