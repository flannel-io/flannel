package remote

import (
	log "github.com/coreos/flannel/Godeps/_workspace/src/github.com/golang/glog"
	"net/http"
)

type httpResp struct {
	writer http.ResponseWriter
	status int
}

func (r *httpResp) Header() http.Header {
	return r.writer.Header()
}

func (r *httpResp) Write(d []byte) (int, error) {
	return r.writer.Write(d)
}

func (r *httpResp) WriteHeader(status int) {
	r.status = status
	r.writer.WriteHeader(status)
}

type httpLoggerHandler struct {
	h http.Handler
}

func (lh httpLoggerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp := &httpResp{w, 0}
	lh.h.ServeHTTP(resp, r)
	log.Infof("%v %v - %v", r.Method, r.RequestURI, resp.status)
}

func httpLogger(h http.Handler) http.Handler {
	return httpLoggerHandler{h}
}
