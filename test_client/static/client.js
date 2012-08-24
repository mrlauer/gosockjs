$(function() {
    var enabledProtocols = {
        'xhr-streaming' : true, 'xhr-polling' : true
    };

    var protocolCheck = function(p) {
        return $("#enable-" + p);
    }

    var i, p;
    for(i in protocols) {
        (function(p) {
            var checkbox = protocolCheck(p);
            if(p in enabledProtocols) {
                checkbox.attr('checked', true);
            }
            checkbox.click(function() {
                if ($(this).is(':checked')) {
                    enabledProtocols[p] = true;
                } else {
                    delete enabledProtocols[p];
                }
            });
        })(protocols[i]);
    }

    var output = function(msg) {
        $("#output").append('<p>' + msg + '</p>')
    };

    var sock;
    var newSock = function() {
        var whitelist = [];
        var i, p;
        for (i in protocols) {
            p = protocols[i];
            if(enabledProtocols[p]) {
                whitelist.push(p);
            }
        }
        sock = new SockJS('/echo', null, {
            protocols_whitelist: whitelist
        });
        sock.onopen = function() {
            output("Opened socket with protocol " + sock.protocol);
            sock.send("Ohai!");
            sock.send("Second send");
        };
        sock.onclose = function(e) {
            output("Byebye! " +  e.code + ', ' + e.reason)
        }
        sock.onmessage = function(e) {
            output(e.data);
        };

    };
    $('#send').click(function() {
        if(sock) {
            sock.send("Boo!");
        }
    });

    $('#close').click(function() {
        if(sock) {
            sock.close();
            sock = null;
        }
    });
    $('#restart').click(function() {
        if(sock) {
            sock.close();
        }
        newSock();
    });

});
