/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package elasticsearch

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

type ESAPIV8 struct {
	ESAPIV7_7
}


func (c *ESAPIV8) InitDefaultTemplate(templateName,indexPrefix string) {
	c.initTemplate(templateName,indexPrefix)
}

func (c *ESAPIV8) getDefaultTemplate(indexPrefix string) string {
	template := `
{
"index_patterns": "%s*",
"settings": {
    "number_of_shards": %v,
    "index.mapping.total_fields.limit": 20000,
    "index.max_result_window":10000000,
	 "analysis": {
		  "analyzer": {
			"suggest_text_search": {
			  "tokenizer": "classic",
			  "filter": [
				"word_delimiter"
			  ]
			}
		  }
		}
  },
  "mappings": {
      "dynamic_templates": [
        {
          "strings": {
            "match_mapping_type": "string",
            "mapping": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        }
      ]
  }
}
`
	return fmt.Sprintf(template, indexPrefix, 1)
}

func (c *ESAPIV8) initTemplate(templateName,indexPrefix string) {
	if global.Env().IsDebug {
		log.Trace("init elasticsearch template")
	}
	if templateName==""{
		templateName = global.Env().GetAppLowercaseName()
	}
	exist, err := c.TemplateExists(templateName)
	if err != nil {
		panic(err)
	}
	if !exist {
		template := c.getDefaultTemplate(indexPrefix)
		log.Trace("template: ", template)
		res, err := c.PutTemplate(templateName, []byte(template))
		if err != nil {
			panic(err)
		}
		if global.Env().IsDebug {
			log.Trace("put template response, %v", string(res))
		}
	}
	log.Debugf("elasticsearch template successful initialized")

}

// Delete used to delete document by id
func (c *ESAPIV8) Delete(indexName,docType, id string, refresh ...string) (*elastic.DeleteResponse, error) {
	indexName=util.UrlEncode(indexName)

	url := c.GetEndpoint() + "/" + indexName + "/_doc/" + id

	if len(refresh)>0 {
		url = url + "?refresh=" + refresh[0]
	}

	resp, err := c.Request(nil, util.Verb_DELETE, url, nil)

	if err != nil {
		return nil, err
	}

	esResp := &elastic.DeleteResponse{}
	esResp.StatusCode=resp.StatusCode
	esResp.RawResult=resp
	err = json.Unmarshal(resp.Body, esResp)

	if err != nil {
		return &elastic.DeleteResponse{}, err
	}
	if esResp.Result != "deleted" && esResp.Result!="not_found" {
		return nil, errors.New(string(resp.Body))
	}
	return esResp, nil
}

// Get fetch document by id
func (c *ESAPIV8) Get(indexName, docType, id string) (*elastic.GetResponse, error) {

	indexName=util.UrlEncode(indexName)

	url := c.GetEndpoint() + "/" + indexName + "/_doc/" + id

	resp, err := c.Request(nil, util.Verb_GET, url, nil)

	esResp := &elastic.GetResponse{}
	if err != nil {
		return nil, err
	}

	esResp.StatusCode=resp.StatusCode
	esResp.RawResult=resp

	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return esResp, err
	}

	return esResp, nil
}

// IndexDoc index a document into elasticsearch
func (c *ESAPIV8) Index(indexName, docType string, id interface{}, data interface{}, refresh string) (*elastic.InsertResponse, error) {

	indexName=util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/_doc/%s", c.GetEndpoint(), indexName, id)

	if id==""{
		url = fmt.Sprintf("%s/%s/_doc", c.GetEndpoint(), indexName)
	}
	if refresh != "" {
		url = fmt.Sprintf("%s?refresh=%s", url, refresh)
	}
	var (
		js []byte
		err error
	)
	if dataBytes, ok := data.([]byte); ok {
		js = dataBytes
	}else{
		js, err = json.Marshal(data)
	}

	if global.Env().IsDebug {
		log.Debug("indexing doc: ", url, ",", string(js))
	}

	if err != nil {
		return nil, err
	}

	resp, err := c.Request(nil, util.Verb_POST, url, js)
	if err != nil {
		return nil, err
	}

	if global.Env().IsDebug {
		log.Trace("indexing response: ", string(resp.Body))
	}

	esResp := &elastic.InsertResponse{}
	err = json.Unmarshal(resp.Body, esResp)
	if err != nil {
		return &elastic.InsertResponse{}, err
	}
	if !(esResp.Result == "created" || esResp.Result == "updated") {
		return nil, errors.New(string(resp.Body))
	}

	return esResp, nil
}

func (c *ESAPIV8) UpdateMapping(indexName string, mappings []byte) ([]byte, error) {
	indexName=util.UrlEncode(indexName)

	url := fmt.Sprintf("%s/%s/_mapping", c.GetEndpoint(), indexName)
	resp, err := c.Request(nil, util.Verb_POST, url, mappings)

	if err != nil {
		panic(err)
	}

	return resp.Body, err
}