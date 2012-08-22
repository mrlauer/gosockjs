package gosockjs

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
)

var iframeFmt = `<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="X-UA-Compatible" content="IE=edge" />
  <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
  <script>
    document.domain = document.domain;
    _sockjs_onload = function(){SockJS.bootstrap_iframe();};
  </script>
  <script src="%s"></script>
</head>
<body>
  <h2>Don't panic!</h2>
  <p>This is a SockJS hidden iframe. It's used for cross domain magic.</p>
</body>
</html>`

func iframeHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	iframe := fmt.Sprintf(iframeFmt, "http://cdn.sockjs.org/sockjs-0.3.min.js")
	md5 := md5.New()
	io.WriteString(md5, iframe)
	qmd5 := fmt.Sprintf(`"%x"`, md5.Sum(nil))

	if req.Header.Get("if-none-match") == qmd5 {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("content-type", "text/html; charset=UTF-8")
	writeCacheAndExpires(w, req)

	w.Header().Set("ETag", qmd5)

	io.WriteString(w, iframe)
}
