gosockjs
========

A Go language implementation of a [SockJS](https://github.com/sockjs/sockjs-client) server.
The protocol is partly specified in a [test suite](https://github.com/sockjs/sockjs-protocol/blob/master/sockjs-protocol-0.3.py). Read that for details.

Only the websocket, raw-websocket, xhr, and xhr_streaming protocols are supported.

Currently UNDER CONSTRUCTION. It is not ready for use.

Some obvious TODOs:
* Timer events: heartbeat and timeout.
* Other protocols.
* Real testing.
