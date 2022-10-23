/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package proxy

import (
	"bytes"
	"crypto/tls"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/agent"
	"infini.sh/framework/lib/fasthttp"
	"io"
	"time"
)

type Request struct {
	Endpoint string
	Path string
	Method string
	BasicAuth agent.BasicAuth
	Body interface{}
	ContentLength int
}

type Response struct {
	Body []byte
	StatusCode int
}

var defaultClient = &fasthttp.Client{
	MaxConnsPerHost: 1000,
	TLSConfig:       &tls.Config{InsecureSkipVerify: true},
	ReadTimeout: time.Second * 3,
}

func DoProxyRequest(req *Request) (*Response, error){
	var (
		freq = fasthttp.AcquireRequest()
		fres = fasthttp.AcquireResponse()
	)
	defer func() {
		fasthttp.ReleaseRequest(freq)
		fasthttp.ReleaseResponse(fres)
	}()
	freq.SetRequestURI(req.Endpoint+ req.Path)
	freq.Header.SetMethod(req.Method)
	if req.BasicAuth.Username != ""{
		freq.SetBasicAuth(req.BasicAuth.Username, req.BasicAuth.Password)
	}
	if req.Body != nil {
		switch req.Body.(type) {
		case []byte:
			freq.SetBody(req.Body.([]byte))
		case string:
			freq.SetBody([]byte(req.Body.(string)))
		case io.Reader:
			freq.SetBodyStream(req.Body.(io.Reader), req.ContentLength)
		default:
			rw := &bytes.Buffer{}
			enc := json.NewEncoder(rw)
			err := enc.Encode(req.Body)
			if err != nil {
				return nil, err
			}
			freq.SetBody(rw.Bytes())
		}
	}

	err := defaultClient.Do(freq, fres)
	if err != nil {
		return nil, err
	}

	return &Response{
		Body: fres.Body(),
		StatusCode: fres.StatusCode(),
	}, nil

}