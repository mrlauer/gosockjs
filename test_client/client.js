$(function() {
    var sock = new SockJS('/echo', null, {
	options: ['xhr-polling']
    });
    sock.onopen = function() {
	console.log('open');
	sock.send("Ohai!");
    };
    sock.onmessage = function(e) {
	console.log('message', e.data);
    };

});
