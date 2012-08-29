gosockjs
========

A Go language implementation of a [SockJS](https://github.com/sockjs/sockjs-client) server.
The protocol is partly specified in a [test suite](https://github.com/sockjs/sockjs-protocol/blob/master/sockjs-protocol-0.3.py). Read that for details. To run the suite against gosockjs, go run test_server/server.go

Supported protocols:
* websocket
* xhr-xtreaming
* iframe-eventsource
* iframe-htmlfile
* xhr-polling
* iframe-xhr-polling
* jsonp-polling

The xdr protocols may work, but have not been tested. Raw-websocket also works.

Websocket version 7 is not supported. Nor is HTML 1.0.

UNDER CONSTRUCTION. Do not lightly assume that it works!

There is a test server in test_server that can be used with the sockjs-protocol suite (go run test_server/server.go). There is a simple client that acts as a quick smoke/sanity test in test_client.

Some TODOs and issues:
* Bulletproof thread issues.
* Real testing. There are some tests here, but not nearly enough. The sockjs-protocol tests are not at all thorough.
* What about https?
* The polling and streaming protocols do not allow keep-alive. That is due to limitations in the Go net and http packages -- or at any rate that's what it looks like to me. Workarounds are possible but are not obviously a good idea.

Tests that are currently failing from the protocol test suite:
* WebsocketHixie76.test_haproxy. Fixing this would mean changes to (a fork of) go.net/websocket.
* Http10.test_streaming.
* WebsocketHybi10.test_firefox_602_connection_header. That test uses websocket version 7, not supported by go.net/websocket.

I have no plans to fix any of these, as I see little point in supporting http1.0 and antique versions of Firefox.
