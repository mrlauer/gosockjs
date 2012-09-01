/*global $, protocols, SockJS*/

$(function() {
    var enabledProtocols = {
        'xhr-streaming' : true, 'xhr-polling' : true
    };

    var protocolCheck = function(p) {
        return $("#enable-" + p);
    };

    var i;
    for(i in protocols) {
        (function(p) {
            var checkbox = protocolCheck(p);
            if(enabledProtocols[p]) {
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
        $("#output").append('<p>' + msg + '</p>');
    };

    var clear = function() {
        $('#output').html('');
    };

    var tryAll = function() {
        var i, p;
        $('label span.result').addClass("indeterminate")
            .removeClass('success')
            .removeClass('failure')
            .text('??');
        for(i in protocols) {
            p = protocols[i];
            (function(i, p) {
                var whitelist = [p];
                var success = undefined;
                var text = 'Some text';
                var textReceived = false;
                var sock = new SockJS('/echo', null, {
                    protocols_whitelist: whitelist
                });
                sock.onopen = function() {
                    success = true;
                    sock.send(text);
                };
                sock.onmessage = function(e) {
                    if(e.data === text) {
                        textReceived = true;
                        sock.close();
                    }
                };
                sock.onclose = function(e) {
                    var result = $("label[for=enable-" + p + "] span");
                    if(success === undefined) {
                        success = false;
                    }

                    result.removeClass("indeterminate");
                    if(success && textReceived) {
                        result.text("succeeded").addClass("success");
                    } else if(success) {
                        result.text("opened, wrong text").addClass("failure");
                    } else {
                        result.text("failed").addClass("failure");
                    }
                };
            })(i, protocols[i]);
        }
    };
    tryAll();

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
            output("Byebye! " +  e.code + ', ' + e.reason);
        };
        sock.onmessage = function(e) {
            output(e.data);
        };

    };
    $('#send').click(function() {
        var text = $('#test-text').val();
        if(sock) {
            sock.send(text);
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
    $('#clear').click(clear);
    $('#retest').click(tryAll);

});
