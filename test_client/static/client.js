$(function() {
    var output = function(msg) {
        $("#output").append('<p>' + msg + '</p>')
    };

    $('#send').click(function() {
        if(sock) {
            sock.send("Boo!");
        }
    });

    var sock = new SockJS('/echo', null, {
	protocols_whitelist: ['xhr-polling', 'xhr-streaming']
    });
    sock.onopen = function() {
	sock.send("Ohai!");
	sock.send("Second send");
    };
    sock.onclose = function(e) {
        output("Byebye! " +  e.code + ', ' + e.reason)
    }
    sock.onmessage = function(e) {
	output(e.data);
    };

});
