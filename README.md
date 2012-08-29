gosockjs
========

A Go language implementation of a [SockJS](https://github.com/sockjs/sockjs-client) server.
The protocol is partly specified in a [test suite](https://github.com/sockjs/sockjs-protocol/blob/master/sockjs-protocol-0.3.py). Read that for details.

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

Some obvious TODOs:
* Html protocol.
* Bulletproof thread issues.
* Real testing. There are some tests here, but not nearly enough. The sockjs-protocol tests are not at all thorough.
