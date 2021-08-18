package api

import (
	"crypto/tls"
	"fmt"
	"net/http"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/lib/fasthttp"
	"strings"
)

func (h *APIHandler) HandleProxyAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string]interface{}{
	}
	targetClusterID := ps.ByName("id")
	method := h.GetParameterOrDefault(req, "method", "")
	path := h.GetParameterOrDefault(req, "path", "")
	if method == "" || path == ""{
		resBody["error"] = fmt.Errorf("parameter method and path is required")
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	exists,_,err:=h.GetClusterClient(targetClusterID)

	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}

	if !exists{
		resBody["error"] = fmt.Sprintf("cluster [%s] not found",targetClusterID)
		h.WriteJSON(w, resBody, http.StatusNotFound)
		return
	}

	var (
		freq = fasthttp.AcquireRequest()
		fres = fasthttp.AcquireResponse()
	)
	defer func() {
		fasthttp.ReleaseRequest(freq)
		fasthttp.ReleaseResponse(fres)
	}()
	config := elastic.GetConfig(targetClusterID)
	if config.BasicAuth != nil {
		freq.SetBasicAuth(config.BasicAuth.Username, config.BasicAuth.Password)
	}
	freq.SetRequestURI(fmt.Sprintf("%s/%s", config.Endpoint, path))
	method = strings.ToUpper(method)
	freq.Header.SetMethod(method)
	freq.SetBodyStream(req.Body, -1)
	defer req.Body.Close()
	client := &fasthttp.Client{
		MaxConnsPerHost: 1000,
		TLSConfig:       &tls.Config{InsecureSkipVerify: true},
	}
	client.Do(freq, fres)
	b := fres.Body()
	w.Header().Set("Content-type", string(fres.Header.ContentType()))
	w.Write(b)
}