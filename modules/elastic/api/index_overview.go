/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	"fmt"
	log "github.com/cihub/seelog"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/adapter"
	"infini.sh/framework/modules/elastic/common"
	"net/http"
	"strings"
	"time"
)

func (h *APIHandler) SearchIndexMetadata(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody:=util.MapStr{}
	reqBody := struct{
		Keyword string `json:"keyword"`
		Size int `json:"size"`
		From int `json:"from"`
		Aggregations []elastic.SearchAggParam `json:"aggs"`
		Highlight elastic.SearchHighlightParam `json:"highlight"`
		Filter elastic.SearchFilterParam `json:"filter"`
		Sort []string `json:"sort"`
		SearchField string `json:"search_field"`
	}{}
	err := h.DecodeJSON(req, &reqBody)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
		return
	}
	aggs := elastic.BuildSearchTermAggregations(reqBody.Aggregations)
	aggs["term_cluster_id"] = util.MapStr{
		"terms": util.MapStr{
			"field": "metadata.cluster_id",
			"size": 1000,
		},
		"aggs": util.MapStr{
			"term_cluster_name": util.MapStr{
				"terms": util.MapStr{
					"field": "metadata.cluster_name",
					"size": 1,
				},
			},
		},
	}
	filter := elastic.BuildSearchTermFilter(reqBody.Filter)
	var should []util.MapStr
	if reqBody.SearchField != ""{
		should = []util.MapStr{
			{
				"prefix": util.MapStr{
					reqBody.SearchField: util.MapStr{
						"value": reqBody.Keyword,
						"boost": 20,
					},
				},
			},
			{
				"match": util.MapStr{
					reqBody.SearchField: util.MapStr{
						"query":                reqBody.Keyword,
						"fuzziness":            "AUTO",
						"max_expansions":       10,
						"prefix_length":        2,
						"fuzzy_transpositions": true,
						"boost":                2,
					},
				},
			},
		}
	}else{
		if reqBody.Keyword != ""{
			should = []util.MapStr{
				{
					"prefix": util.MapStr{
						"metadata.index_name": util.MapStr{
							"value": reqBody.Keyword,
							"boost": 30,
						},
					},
				},
				{
					"prefix": util.MapStr{
						"metadata.aliases": util.MapStr{
							"value": reqBody.Keyword,
							"boost": 20,
						},
					},
				},
				{
					"match": util.MapStr{
						"search_text": util.MapStr{
							"query":                reqBody.Keyword,
							"fuzziness":            "AUTO",
							"max_expansions":       10,
							"prefix_length":        2,
							"fuzzy_transpositions": true,
							"boost":                2,
						},
					},
				},
				{
					"query_string": util.MapStr{
						"fields":                 []string{"*"},
						"query":                  reqBody.Keyword,
						"fuzziness":              "AUTO",
						"fuzzy_prefix_length":    2,
						"fuzzy_max_expansions":   10,
						"fuzzy_transpositions":   true,
						"allow_leading_wildcard": false,
					},
				},
			}
		}
	}

	must := []interface{}{
	}
	hasAllPrivilege, indexPrivilege := h.GetCurrentUserIndex(req)
	if !hasAllPrivilege && len(indexPrivilege) == 0 {
		h.WriteJSON(w, elastic.SearchResponse{

		}, http.StatusOK)
		return
	}
	if !hasAllPrivilege {
		indexShould := make([]interface{}, 0, len(indexPrivilege))
		for clusterID, indices := range indexPrivilege {
			var (
				wildcardIndices []string
				normalIndices []string
			)
			for _, index := range indices {
				if strings.Contains(index,"*") {
					wildcardIndices = append(wildcardIndices, index)
					continue
				}
				normalIndices = append(normalIndices, index)
			}
			subShould := []util.MapStr{}
			if len(wildcardIndices) > 0 {
				subShould = append(subShould, util.MapStr{
					"query_string": util.MapStr{
						"query": strings.Join(wildcardIndices, " "),
						"fields": []string{"metadata.index_name"},
						"default_operator": "OR",
					},
				})
			}
			if len(normalIndices) > 0 {
				subShould = append(subShould, util.MapStr{
					"terms": util.MapStr{
						"metadata.index_name": normalIndices,
					},
				})
			}
			indexShould = append(indexShould, util.MapStr{
				"bool": util.MapStr{
					"must": []util.MapStr{
						{
							"wildcard": util.MapStr{
								"metadata.cluster_id": util.MapStr{
									"value": clusterID,
								},
							},
						},
						{
							"bool": util.MapStr{
								"minimum_should_match": 1,
								"should": subShould,
							},
						},
					},
				},
			})
		}
		indexFilter := util.MapStr{
			"bool": util.MapStr{
				"minimum_should_match": 1,
				"should": indexShould,
			},
		}
		must = append(must, indexFilter)
	}
	boolQuery := util.MapStr{
		"must_not": []util.MapStr{
			{
				"term": util.MapStr{
					"metadata.labels.index_status": "deleted",
				},
			},
		},
		"filter": filter,
		"must": must,
	}
	if len(should) > 0 {
		boolQuery["should"] = should
		boolQuery["minimum_should_match"] = 1
	}
	query := util.MapStr{
		"aggs":      aggs,
		"size":      reqBody.Size,
		"from": reqBody.From,
		"highlight": elastic.BuildSearchHighlight(&reqBody.Highlight),
		"query": util.MapStr{
			"bool": boolQuery,
		},
		"sort": []util.MapStr{
			{
				"timestamp": util.MapStr{
					"order": "desc",
				},
			},
		},
	}
	if len(reqBody.Sort) > 1 {
		query["sort"] =  []util.MapStr{
			{
				reqBody.Sort[0]: util.MapStr{
					"order": reqBody.Sort[1],
				},
			},
		}
	}
	dsl := util.MustToJSONBytes(query)
	response, err := elastic.GetClient(global.MustLookupString(elastic.GlobalSystemElasticsearchID)).SearchWithRawQueryDSL(orm.GetIndexName(elastic.IndexConfig{}), dsl)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
		return
	}
	w.Write(util.MustToJSONBytes(response))

}
func (h *APIHandler) FetchIndexInfo(w http.ResponseWriter,  req *http.Request, ps httprouter.Params) {
	var indexIDs []interface{}
	h.DecodeJSON(req, &indexIDs)

	if len(indexIDs) == 0 {
		h.WriteJSON(w, util.MapStr{}, http.StatusOK)
		return
	}
	// map indexIDs(cluster_id:index_name => cluster_uuid:indexName)
	var (
		indexIDM = map[string]string{}
		newIndexIDs []interface{}
		clusterIndexNames = map[string][]string{}
	)
	for _, indexID := range indexIDs {
		if v, ok := indexID.(string); ok {
			parts := strings.Split(v, ":")
			if len(parts) != 2 {
				log.Warnf("got wrong index_id: %s", v)
				continue
			}
			clusterIndexNames[parts[0]] = append(clusterIndexNames[parts[0]], parts[1])
		}
	}
	for clusterID, indexNames := range clusterIndexNames {
		clusterUUID, err  := adapter.GetClusterUUID(clusterID)
		if err != nil {
			log.Warnf("get cluster uuid error: %v", err)
			continue
		}
		for _, indexName := range indexNames {
			newIndexID := fmt.Sprintf("%s:%s", clusterUUID, indexName)
			newIndexIDs = append(newIndexIDs, newIndexID)
			indexIDM[fmt.Sprintf("%s:%s", clusterID, indexName)] = newIndexID
		}
	}
	q1 := orm.Query{WildcardIndex: true}
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.name", "shard_stats"),
		orm.In("metadata.labels.index_id", newIndexIDs),
	)
	q1.Collapse("metadata.labels.shard_id")
	q1.AddSort("timestamp", orm.DESC)
	q1.Size = 20000

	err, results := orm.Search(&event.Event{}, &q1)
	if err != nil {
		h.WriteJSON(w, util.MapStr{
			"error": err.Error(),
		}, http.StatusInternalServerError)
	}

	summaryMap := map[string]*ShardsSummary{}
	for _, hit := range results.Result {
		if hitM, ok := hit.(map[string]interface{}); ok {
			shardDocCount, _ := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "shard_stats", "docs", "count"}, hitM)
			shardDocDeleted, _ := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "shard_stats", "docs", "deleted"}, hitM)
			storeInBytes, _ := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "shard_stats", "store", "size_in_bytes"}, hitM)
			indexID, _ := util.GetMapValueByKeys([]string{"metadata", "labels", "index_id"}, hitM)
			indexName, _ := util.GetMapValueByKeys([]string{"metadata", "labels", "index_name"}, hitM)
			primary, _ := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "shard_stats", "routing", "primary"}, hitM)
			if v, ok := indexID.(string); ok {
				if _, ok = summaryMap[v]; !ok {
					summaryMap[v] = &ShardsSummary{}
				}
				indexInfo := summaryMap[v]
				if iv, ok := indexName.(string); ok {
					indexInfo.Index = iv
				}
				if count, ok := shardDocCount.(float64); ok && primary == true {
					indexInfo.DocsCount += int64(count)
				}
				if deleted, ok := shardDocDeleted.(float64); ok && primary == true {
					indexInfo.DocsDeleted += int64(deleted)
				}
				if storeSize, ok := storeInBytes.(float64); ok {
					indexInfo.StoreInBytes += int64(storeSize)
					if primary == true {
						indexInfo.PriStoreInBytes += int64(storeSize)
					}
				}
				if primary == true {
					indexInfo.Shards++
				}else{
					indexInfo.Replicas++
				}
				indexInfo.Timestamp = hitM["timestamp"]
			}
		}
	}

	statusMetric, err := getIndexStatusOfRecentDay(indexIDs)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	bucketSize, min, max, err := h.getMetricRangeAndBucketSize(req, 60, (15))
	if err != nil {
		panic(err)
		return
	}
	// 索引速率
	indexMetric:=newMetricItem("indexing", 1, OperationGroupKey)
	indexMetric.OnlyPrimary = true
	indexMetric.AddAxi("indexing rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems := []GroupMetricItem{}
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "indexing",
		Field: "payload.elasticsearch.shard_stats.indexing.index_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: indexMetric,
		FormatType: "num",
		Units: "Indexing/s",
	})
	queryMetric:=newMetricItem("search", 2, OperationGroupKey)
	queryMetric.AddAxi("query rate","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	nodeMetricItems=append(nodeMetricItems, GroupMetricItem{
		Key: "search",
		Field: "payload.elasticsearch.shard_stats.search.query_total",
		ID: util.GetUUID(),
		IsDerivative: true,
		MetricItem: queryMetric,
		FormatType: "num",
		Units: "Search/s",
	})

	aggs:=map[string]interface{}{}
	query :=map[string]interface{}{}
	query["query"]=util.MapStr{
		"bool": util.MapStr{
			"must":  []util.MapStr{
				{
					"term": util.MapStr{
						"metadata.category": util.MapStr{
							"value": "elasticsearch",
						},
					},
				},
				{
					"term": util.MapStr{
						"metadata.name": util.MapStr{
							"value": "shard_stats",
						},
					},
				},
				{
					"terms": util.MapStr{
						"metadata.labels.index_id": newIndexIDs,
					},
				},
			},
			"filter": []util.MapStr{
				{
					"range": util.MapStr{
						"timestamp": util.MapStr{
							"gte": min,
							"lte": max,
						},
					},
				},
			},
		},
	}

	sumAggs := util.MapStr{}
	for _,metricItem:=range nodeMetricItems{
		leafAgg := util.MapStr{
			"max":util.MapStr{
				"field": metricItem.Field,
			},
		}
		var sumBucketPath = "term_shard>"+ metricItem.ID
		if metricItem.MetricItem.OnlyPrimary {
			filterSubAggs := util.MapStr{
				metricItem.ID: leafAgg,
			}
			aggs["filter_pri"]=util.MapStr{
				"filter": util.MapStr{
					"term": util.MapStr{
						"payload.elasticsearch.shard_stats.routing.primary": util.MapStr{
							"value": true,
						},
					},
				},
				"aggs": filterSubAggs,
			}
			sumBucketPath = "term_shard>filter_pri>"+ metricItem.ID
		}else{
			aggs[metricItem.ID] = leafAgg
		}

		sumAggs[metricItem.ID] = util.MapStr{
			"sum_bucket": util.MapStr{
				"buckets_path": sumBucketPath,
			},
		}
		if metricItem.IsDerivative{
			sumAggs[metricItem.ID+"_deriv"]=util.MapStr{
				"derivative":util.MapStr{
					"buckets_path": metricItem.ID,
				},
			}
		}
	}
	sumAggs["term_shard"]= util.MapStr{
		"terms": util.MapStr{
			"field": "metadata.labels.shard_id",
			"size": 10000,
		},
		"aggs": aggs,
	}

	bucketSizeStr := fmt.Sprintf("%ds", bucketSize)
	intervalField, err := getDateHistogramIntervalField(global.MustLookupString(elastic.GlobalSystemElasticsearchID), bucketSizeStr)
	if err != nil {
		panic(err)
	}
	query["size"]=0
	query["aggs"]= util.MapStr{
		"group_by_level": util.MapStr{
			"terms": util.MapStr{
				"field": "metadata.labels.index_id",
				"size":  100,
			},
			"aggs": util.MapStr{
				"dates": util.MapStr{
					"date_histogram":util.MapStr{
						"field": "timestamp",
						intervalField: bucketSizeStr,
					},
					"aggs":sumAggs,
				},
			},
		},
	}
	metrics := h.getMetrics(query, nodeMetricItems, bucketSize)
	indexMetrics := map[string]util.MapStr{}
	for key, item := range metrics {
		for _, line := range item.Lines {
			if _, ok := indexMetrics[line.Metric.Label]; !ok{
				indexMetrics[line.Metric.Label] = util.MapStr{
				}
			}
			indexMetrics[line.Metric.Label][key] = line.Data
		}
	}
	infos := util.MapStr{}
	for _, tempIndexID := range indexIDs {
		result := util.MapStr{}

		indexID := tempIndexID.(string)
		newIndexID := indexIDM[indexID]

		result["summary"] = summaryMap[newIndexID]
		result["metrics"] = util.MapStr{
			"status": util.MapStr{
				"metric": util.MapStr{
					"label": "Recent Index Status",
					"units": "day",
				},
				"data": statusMetric[indexID],
			},
			"indexing": util.MapStr{
				"metric": util.MapStr{
					"label": "Indexing",
					"units": "s",
				},
				"data": indexMetrics[newIndexID]["indexing"],
			},
			"search": util.MapStr{
				"metric": util.MapStr{
					"label": "Search",
					"units": "s",
				},
				"data": indexMetrics[newIndexID]["search"],
			},
		}
		infos[indexID] = result
	}
	h.WriteJSON(w, infos, http.StatusOK)
}

func (h *APIHandler) GetIndexInfo(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	clusterID := ps.MustGetParameter("id")
	indexID := ps.MustGetParameter("index")
	parts := strings.Split(indexID, ":")
	if len(parts) > 1 && !h.IsIndexAllowed(req, clusterID, parts[1]) {
		h.WriteError(w,  http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if len(parts) < 2 {
		h.WriteError(w, "invalid index id: "+ indexID, http.StatusInternalServerError)
		return
	}

	q := orm.Query{
		Size: 1,
	}
	q.Conds = orm.And(orm.Eq("metadata.index_name", parts[1]), orm.Eq("metadata.cluster_id", clusterID))
	q.AddSort("timestamp", orm.DESC)

	err, res := orm.Search(&elastic.IndexConfig{}, &q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := elastic.SearchResponse{}
	util.FromJSONBytes(res.Raw, &response)
	if len(response.Hits.Hits) == 0 {
		log.Warnf("index [%v][%v] not found, may be you should wait several seconds", clusterID, indexID)
		h.WriteJSON(w, util.MapStr{}, http.StatusOK)
		return
	}
	clusterUUID, err := adapter.GetClusterUUID(clusterID)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	q1 := orm.Query{
		Size: 1000,
		WildcardIndex: true,
	}
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.name", "shard_stats"),
		orm.Eq("metadata.labels.index_name", parts[1]),
		orm.Eq("metadata.labels.cluster_uuid", clusterUUID),
		orm.Ge("timestamp", "now-15m"),
	)
	q1.Collapse("metadata.labels.shard_id")
	q1.AddSort("timestamp", orm.DESC)
	err, result := orm.Search(&event.Event{}, &q1)
	summary := util.MapStr{}
	hit := response.Hits.Hits[0].Source
	var (
		shardsNum int
		replicasNum int
		indexInfo = util.MapStr{
			"index": parts[1],
		}
	)
	if aliases, ok := util.GetMapValueByKeys([]string{"metadata", "aliases"}, hit); ok {
		health, _ := util.GetMapValueByKeys([]string{"metadata", "labels", "health_status"}, hit)
		state, _ := util.GetMapValueByKeys([]string{"metadata", "labels", "state"}, hit)
		shards, _ := util.GetMapValueByKeys([]string{"payload", "index_state", "settings", "index", "number_of_shards"}, hit)
		replicas, _ := util.GetMapValueByKeys([]string{"payload", "index_state", "settings", "index", "number_of_replicas"}, hit)
		shardsNum, _ = util.ToInt(shards.(string))
		replicasNum, _ = util.ToInt(replicas.(string))
		summary["aliases"] = aliases
		summary["timestamp"] = hit["timestamp"]
		if state == "delete" {
			health = "N/A"
		}
		indexInfo["health"] = health
		indexInfo["status"] = state
	}
	if len(result.Result) > 0 {
		shardSum := ShardsSummary{}
		for _, row := range result.Result {
			resultM, ok := row.(map[string]interface{})
			if ok {
				primary, _ := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "shard_stats", "routing", "primary"}, resultM)
				storeInBytes, _ := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "shard_stats", "store", "size_in_bytes"}, resultM)
				if docs, ok := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "shard_stats", "docs", "count"}, resultM); ok {
					//summary["docs"] = docs
					if v, ok := docs.(float64); ok && primary == true{
						shardSum.DocsCount += int64(v)
					}
				}
				if storeSize, ok := storeInBytes.(float64); ok {
					shardSum.StoreInBytes += int64(storeSize)
					if primary == true {
						shardSum.PriStoreInBytes += int64(storeSize)
					}
				}
				if primary == true {
					shardSum.Shards++
				}else{
					shardSum.Replicas++
				}
			}
			summary["timestamp"] = resultM["timestamp"]
		}
		indexInfo["docs_count"] = shardSum.DocsCount
		indexInfo["pri_store_size"] = util.FormatBytes(float64(shardSum.PriStoreInBytes), 1)
		indexInfo["store_size"] = util.FormatBytes(float64(shardSum.StoreInBytes), 1)
		indexInfo["shards"] = shardSum.Shards + shardSum.Replicas

		summary["unassigned_shards"] = (replicasNum + 1) * shardsNum - shardSum.Shards - shardSum.Replicas
	}
	summary["index_info"] = indexInfo

	h.WriteJSON(w, summary, http.StatusOK)
}

func (h *APIHandler) GetIndexShards(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	clusterID := ps.MustGetParameter("id")
	indexName := ps.MustGetParameter("index")
	q1 := orm.Query{
		Size: 1000,
		WildcardIndex: true,
	}
	clusterUUID, err := adapter.GetClusterUUID(clusterID)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.name", "shard_stats"),
		orm.Eq("metadata.labels.index_name", indexName),
		orm.Eq("metadata.labels.cluster_uuid", clusterUUID),
		orm.Ge("timestamp", "now-15m"),
	)
	q1.Collapse("metadata.labels.shard_id")
	q1.AddSort("timestamp", orm.DESC)
	err, result := orm.Search(&event.Event{}, &q1)
	if err != nil {
		log.Error(err)
		h.WriteError(w,err.Error(), http.StatusInternalServerError )
		return
	}
	var shardInfo interface{} = []interface{}{}
	if len(result.Result) > 0 {
		q := &orm.Query{
			Size: 500,
		}
		q.Conds = orm.And(
			orm.Eq("metadata.cluster_id", clusterID),
		)
		err, nodesResult := orm.Search(elastic.NodeConfig{}, q)
		if err != nil {
			log.Error(err)
			h.WriteError(w,err.Error(), http.StatusInternalServerError )
			return
		}
		nodeIDToName := util.MapStr{}
		for _, row := range nodesResult.Result {
			if rowM, ok := row.(map[string]interface{}); ok {
				nodeName, _ := util.MapStr(rowM).GetValue("metadata.node_name")
				nodeID, _ := util.MapStr(rowM).GetValue("metadata.node_id")
				if v, ok := nodeID.(string); ok {
					nodeIDToName[v] = nodeName
				}
			}
		}
		for _, item := range result.Result {
			row, ok := item.(map[string]interface{})
			if ok {
				source := util.MapStr(row)
				nodeID, _ := source.GetValue("metadata.labels.node_id")
				if v, ok := nodeID.(string); ok {
					if v, ok := nodeIDToName[v]; ok {
						source.Put("metadata.labels.node_name", v)
					}

				}
				//todo add index qps info
			}
		}
		shardInfo = result.Result
	}

	h.WriteJSON(w, shardInfo, http.StatusOK)
}

func (h *APIHandler) GetSingleIndexMetrics(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	clusterID := ps.MustGetParameter("id")
	indexName := ps.MustGetParameter("index")
	if !h.IsIndexAllowed(req, clusterID, indexName) {
		h.WriteJSON(w, util.MapStr{
			"error": http.StatusText(http.StatusForbidden),
		}, http.StatusForbidden)
		return
	}
	clusterUUID, err := adapter.GetClusterUUID(clusterID)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shardID := h.GetParameterOrDefault(req, "shard_id", "")

	var must = []util.MapStr{
		{
			"term": util.MapStr{
				"metadata.labels.cluster_uuid": util.MapStr{
					"value": clusterUUID,
				},
			},
		},
		{
			"term": util.MapStr{
				"metadata.category": util.MapStr{
					"value": "elasticsearch",
				},
			},
		},
		{
			"term": util.MapStr{
				"metadata.name": util.MapStr{
					"value": "shard_stats",
				},
			},
		},
		{
			"term": util.MapStr{
				"metadata.labels.index_name": util.MapStr{
					"value": indexName,
				},
			},
		},
	}
	if shardID != "" {
		must = append(must, util.MapStr{
			"term": util.MapStr{
				"metadata.labels.shard_id": util.MapStr{
					"value": shardID,
				},
			},
		})
	}
	resBody := map[string]interface{}{}
	bucketSize, min, max, err := h.getMetricRangeAndBucketSize(req, 10, 60)
	if err != nil {
		log.Error(err)
		resBody["error"] = err
		h.WriteJSON(w, resBody, http.StatusInternalServerError)
		return
	}
	meta := elastic.GetMetadata(clusterID)
	if meta != nil && meta.Config.MonitorConfigs != nil && meta.Config.MonitorConfigs.IndexStats.Interval != "" {
		du, _ := time.ParseDuration(meta.Config.MonitorConfigs.IndexStats.Interval)
		if bucketSize < int(du.Seconds()) {
			bucketSize = int(du.Seconds())
		}
	}
	query := map[string]interface{}{}
	query["query"] = util.MapStr{
		"bool": util.MapStr{
			"must": must,
			"filter": []util.MapStr{
				{
					"range": util.MapStr{
						"timestamp": util.MapStr{
							"gte": min,
							"lte": max,
						},
					},
				},
			},
		},
	}

	bucketSizeStr := fmt.Sprintf("%vs", bucketSize)
	metricItems := []*common.MetricItem{}
	metricItem:=newMetricItem("index_throughput", 1, OperationGroupKey)
	metricItem.AddAxi("indexing","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)
	metricItem.AddLine("Indexing Rate","Total Indexing","Number of documents being indexed for node.","group1","payload.elasticsearch.shard_stats.indexing.index_total","max",bucketSizeStr,"doc/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.AddLine("Deleting Rate","Total Deleting","Number of documents being deleted for node.","group1","payload.elasticsearch.shard_stats.indexing.delete_total","max",bucketSizeStr,"doc/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItems=append(metricItems,metricItem)
	metricItem=newMetricItem("search_throughput", 2, OperationGroupKey)
	metricItem.AddAxi("searching","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,false)
	metricItem.AddLine("Search Rate","Search Rate",
		"Number of search requests being executed.",
		"group1","payload.elasticsearch.shard_stats.search.query_total","max",bucketSizeStr,"query/s","num","0,0.[00]","0,0.[00]",false,true)
	metricItems=append(metricItems,metricItem)

	metricItem=newMetricItem("index_latency", 3, LatencyGroupKey)
	metricItem.AddAxi("indexing","group1",common.PositionLeft,"num","0,0","0,0.[00]",5,true)

	metricItem.AddLine("Indexing Latency","Total Indexing Latency","Average latency for indexing documents.","group1","payload.elasticsearch.shard_stats.indexing.index_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.Lines[0].Metric.Field2 = "payload.elasticsearch.shard_stats.indexing.index_total"
	metricItem.Lines[0].Metric.Calc = func(value, value2 float64) float64 {
		return value/value2
	}
	metricItem.AddLine("Deleting Latency","Total Deleting Latency","Average latency for delete documents.","group1","payload.elasticsearch.shard_stats.indexing.delete_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.Lines[1].Metric.Field2 = "payload.elasticsearch.shard_stats.indexing.delete_total"
	metricItem.Lines[1].Metric.Calc = func(value, value2 float64) float64 {
		return value/value2
	}
	metricItems=append(metricItems,metricItem)

	metricItem=newMetricItem("search_latency", 4, LatencyGroupKey)
	metricItem.AddAxi("searching","group2",common.PositionLeft,"num","0,0","0,0.[00]",5,false)

	metricItem.AddLine("Searching","Query Latency","Average latency for searching query.","group2","payload.elasticsearch.shard_stats.search.query_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.Lines[0].Metric.Field2 = "payload.elasticsearch.shard_stats.search.query_total"
	metricItem.Lines[0].Metric.Calc = func(value, value2 float64) float64 {
		return value/value2
	}
	metricItem.AddLine("Searching","Fetch Latency","Average latency for searching fetch.","group2","payload.elasticsearch.shard_stats.search.fetch_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.Lines[1].Metric.Field2 = "payload.elasticsearch.shard_stats.search.fetch_total"
	metricItem.Lines[1].Metric.Calc = func(value, value2 float64) float64 {
		return value/value2
	}
	metricItem.AddLine("Searching","Scroll Latency","Average latency for searching fetch.","group2","payload.elasticsearch.shard_stats.search.scroll_time_in_millis","max",bucketSizeStr,"ms","num","0,0.[00]","0,0.[00]",false,true)
	metricItem.Lines[2].Metric.Field2 = "payload.elasticsearch.shard_stats.search.scroll_total"
	metricItem.Lines[2].Metric.Calc = func(value, value2 float64) float64 {
		return value/value2
	}
	metricItems=append(metricItems,metricItem)
	metrics := h.getSingleIndexMetrics(metricItems,query, bucketSize)
	healthMetric, err := h.getIndexHealthMetric(clusterID, indexName, min, max, bucketSize)
	if err != nil {
		log.Error(err)
	}
	metrics["index_health"] = healthMetric
	resBody["metrics"] = metrics
	h.WriteJSON(w, resBody, http.StatusOK)
}

func (h *APIHandler) getIndexHealthMetric(id, indexName string, min, max int64, bucketSize int)(*common.MetricItem, error){
	bucketSizeStr:=fmt.Sprintf("%vs",bucketSize)
	intervalField, err := getDateHistogramIntervalField(global.MustLookupString(elastic.GlobalSystemElasticsearchID), bucketSizeStr)
	if err != nil {
		return nil, err
	}
	query := util.MapStr{
		"query": util.MapStr{
			"bool": util.MapStr{
				"must": []util.MapStr{
					{
						"term": util.MapStr{
							"metadata.labels.cluster_id": util.MapStr{
								"value": id,
							},
						},
					},
					{
						"term": util.MapStr{
							"metadata.category": util.MapStr{
								"value": "elasticsearch",
							},
						},
					},
					{
						"term": util.MapStr{
							"metadata.name": util.MapStr{
								"value": "index_stats",
							},
						},
					},
					{
						"term": util.MapStr{
							"metadata.labels.index_name": util.MapStr{
								"value": indexName,
							},
						},
					},
				},
				"filter": []util.MapStr{
					{
						"range": util.MapStr{
							"timestamp": util.MapStr{
								"gte": min,
								"lte": max,
							},
						},
					},
				},
			},
		},
		"aggs": util.MapStr{
			"dates": util.MapStr{
				"date_histogram": util.MapStr{
					"field": "timestamp",
					intervalField: bucketSizeStr,
				},
				"aggs": util.MapStr{
					"group_status": util.MapStr{
						"terms": util.MapStr{
							"field": "payload.elasticsearch.index_stats.index_info.health",
							"size": 5,
						},
					},
				},
			},
		},
	}
	response, err := elastic.GetClient(global.MustLookupString(elastic.GlobalSystemElasticsearchID)).SearchWithRawQueryDSL(getAllMetricsIndex(), util.MustToJSONBytes(query))
	if err != nil {
		log.Error(err)
		return nil, err
	}

	metricItem:=newMetricItem("index_health", 1, "")
	metricItem.AddLine("health","Health","","group1","payload.elasticsearch.index_stats.index_info.health","max",bucketSizeStr,"%","ratio","0.[00]","0.[00]",false,false)

	metricData := []interface{}{}
	if response.StatusCode == 200 {
		metricData, err = parseHealthMetricData(response.Aggregations["dates"].Buckets)
		if err != nil {
			return nil, err
		}
	}
	metricItem.Lines[0].Data = metricData
	metricItem.Lines[0].Type = common.GraphTypeBar
	return metricItem, nil
}


func getIndexStatusOfRecentDay(indexIDs []interface{})(map[string][]interface{}, error){
	q := orm.Query{
		WildcardIndex: true,
	}
	query := util.MapStr{
		"aggs": util.MapStr{
			"group_by_index_id": util.MapStr{
				"terms": util.MapStr{
					"field": "metadata.labels.index_id",
					"size": 100,
				},
				"aggs": util.MapStr{
					"time_histogram": util.MapStr{
						"date_range": util.MapStr{
							"field":     "timestamp",
							"format":    "yyyy-MM-dd",
							"time_zone": "+08:00",
							"ranges": []util.MapStr{
								{
									"from": "now-13d/d",
									"to": "now-12d/d",
								}, {
									"from": "now-12d/d",
									"to": "now-11d/d",
								},
								{
									"from": "now-11d/d",
									"to": "now-10d/d",
								},
								{
									"from": "now-10d/d",
									"to": "now-9d/d",
								}, {
									"from": "now-9d/d",
									"to": "now-8d/d",
								},
								{
									"from": "now-8d/d",
									"to": "now-7d/d",
								},
								{
									"from": "now-7d/d",
									"to": "now-6d/d",
								},
								{
									"from": "now-6d/d",
									"to": "now-5d/d",
								}, {
									"from": "now-5d/d",
									"to": "now-4d/d",
								},
								{
									"from": "now-4d/d",
									"to": "now-3d/d",
								},{
									"from": "now-3d/d",
									"to": "now-2d/d",
								}, {
									"from": "now-2d/d",
									"to": "now-1d/d",
								}, {
									"from": "now-1d/d",
									"to": "now/d",
								},
								{
									"from": "now/d",
									"to": "now",
								},
							},
						},
						"aggs": util.MapStr{
							"term_health": util.MapStr{
								"terms": util.MapStr{
									"field": "payload.elasticsearch.index_stats.index_info.health",
								},
							},
						},
					},
				},
			},
		},
		"sort": []util.MapStr{
			{
				"timestamp": util.MapStr{
					"order": "desc",
				},
			},
		},
		"size": 0,
		"query": util.MapStr{
			"bool": util.MapStr{
				"filter": []util.MapStr{
					{
						"range": util.MapStr{
							"timestamp": util.MapStr{
								"gte": "now-15d",
								"lte": "now",
							},
						},
					},
				},
				"must": []util.MapStr{
					{
						"term": util.MapStr{
							"metadata.name": util.MapStr{
								"value": "index_stats",
							},
						},
					},
					{
						"terms": util.MapStr{
							"metadata.labels.index_id": indexIDs,
						},
					},
				},
			},
		},
	}
	q.RawQuery = util.MustToJSONBytes(query)

	err, res := orm.Search(&event.Event{}, &q)
	if err != nil {
		return nil, err
	}

	response := elastic.SearchResponse{}
	util.FromJSONBytes(res.Raw, &response)
	recentStatus := map[string][]interface{}{}
	for _, bk := range response.Aggregations["group_by_index_id"].Buckets {
		indexKey := bk["key"].(string)
		recentStatus[indexKey] = []interface{}{}
		if histogramAgg, ok := bk["time_histogram"].(map[string]interface{}); ok {
			if bks, ok := histogramAgg["buckets"].([]interface{}); ok {
				for _, bkItem := range  bks {
					if bkVal, ok := bkItem.(map[string]interface{}); ok {
						if termHealth, ok := bkVal["term_health"].(map[string]interface{}); ok {
							if healthBks, ok := termHealth["buckets"].([]interface{}); ok {
								if len(healthBks) == 0 {
									continue
								}
								healthMap := map[string]int{}
								status := "unknown"
								for _, hbkItem := range  healthBks {
									if hitem, ok := hbkItem.(map[string]interface{}); ok {
										healthMap[hitem["key"].(string)] = 1
									}
								}
								if _, ok = healthMap["red"]; ok {
									status = "red"
								}else if _, ok = healthMap["yellow"]; ok {
									status = "yellow"
								}else if _, ok = healthMap["green"]; ok {
									status = "green"
								}
								recentStatus[indexKey] = append(recentStatus[indexKey], []interface{}{bkVal["key"], status})
							}
						}
					}
				}
			}
		}
	}
	return recentStatus, nil
}

func (h *APIHandler) getIndexNodes(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resBody := map[string] interface{}{}
	id := ps.ByName("id")
	indexName := ps.ByName("index")
	if !h.IsIndexAllowed(req, id, indexName) {
		h.WriteJSON(w, util.MapStr{
			"error": http.StatusText(http.StatusForbidden),
		}, http.StatusForbidden)
		return
	}
	q := &orm.Query{ Size: 1}
	q.AddSort("timestamp", orm.DESC)
	q.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.labels.cluster_id", id),
		orm.Eq("metadata.labels.index_name", indexName),
		orm.Eq("metadata.name", "index_routing_table"),
	)

	err, result := orm.Search(event.Event{}, q)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
	}
	namesM := util.MapStr{}
	if len(result.Result) > 0 {
		if data, ok := result.Result[0].(map[string]interface{}); ok {
			if routingTable, exists := util.GetMapValueByKeys([]string{"payload", "elasticsearch", "index_routing_table"}, data); exists {
				if table, ok := routingTable.(map[string]interface{}); ok{
					if shardsM, ok := table["shards"].(map[string]interface{}); ok {
						for _, rows := range shardsM {
							if rowsArr, ok := rows.([]interface{}); ok {
								for _, rowsInner := range rowsArr {
									if rowsInnerM, ok := rowsInner.(map[string]interface{}); ok {
										if v, ok := rowsInnerM["node"].(string); ok {
											namesM[v] = true
										}
									}
								}
							}

						}
					}

				}
			}
		}
	}

	//node uuid
	nodeIds := make([]interface{}, 0, len(namesM) )
	for name, _ := range namesM {
		nodeIds = append(nodeIds, name)
	}

	q1 := &orm.Query{ Size: 100}
	q1.AddSort("timestamp", orm.DESC)
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.cluster_id", id),
		orm.In("metadata.node_id", nodeIds),
	)
	err, result = orm.Search(elastic.NodeConfig{}, q1)
	if err != nil {
		resBody["error"] = err.Error()
		h.WriteJSON(w,resBody, http.StatusInternalServerError )
	}
	nodes := []interface{}{}
	for _, hit := range result.Result {
		if hitM, ok := hit.(map[string]interface{}); ok {
			nodeId, _ := util.GetMapValueByKeys([]string{"metadata", "node_id"}, hitM)
			nodeName, _ := util.GetMapValueByKeys([]string{"metadata", "node_name"}, hitM)
			status, _ := util.GetMapValueByKeys([]string{"metadata", "labels", "status"}, hitM)
			ip, _ := util.GetMapValueByKeys([]string{"metadata", "labels", "ip"}, hitM)
			transportAddress, _ := util.GetMapValueByKeys([]string{"payload", "node_state", "transport_address"}, hitM)
			var port string
			if v, ok := transportAddress.(string); ok {
				parts := strings.Split(v, ":")
				if len(parts) > 1 {
					port = parts[1]
				}
			}

			if v, ok := nodeId.(string); ok {
				ninfo := util.MapStr{
					"id": v,
					"name": nodeName,
					"ip": ip,
					"port": port,
					"status": status,
					"timestamp": hitM["timestamp"],
				}
				nodes = append(nodes, ninfo)
			}
		}
	}

	h.WriteJSON(w, nodes, http.StatusOK)
}

func (h APIHandler) ListIndex(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	clusterIds := h.GetParameterOrDefault(req, "ids", "")
	keyword := h.GetParameterOrDefault(req, "keyword", "")
	ids := strings.Split(clusterIds, ",")
	if len(ids) == 0 {
		h.Error400(w, "cluster id is required")
		return
	}
	var must = []util.MapStr{}

	if !util.StringInArray(ids, "*"){

		must = append(must, util.MapStr{
			"terms": util.MapStr{
				"metadata.cluster_id": ids,
			},
		})
	}

	if keyword != "" {
		must = append(must, util.MapStr{
			"wildcard":util.MapStr{
				"metadata.index_name":
				util.MapStr{"value": fmt.Sprintf("*%s*", keyword)},
			},
		})
	}
	var dsl = util.MapStr{
		"_source": []string{"metadata.index_name"},
		"collapse": util.MapStr{
			"field": "metadata.index_name",
		},
		"size": 100,
		"query": util.MapStr{
			"bool": util.MapStr{
				"must": must,
				"must_not": []util.MapStr{
					{
						"term": util.MapStr{
							"metadata.labels.state": util.MapStr{
								"value": "delete",
							},
						},
					},
				},
			},
		},
	}


	esClient := elastic.GetClient(global.MustLookupString(elastic.GlobalSystemElasticsearchID))
	indexName := orm.GetIndexName(elastic.IndexConfig{})
	resp, err := esClient.SearchWithRawQueryDSL(indexName, util.MustToJSONBytes(dsl))
	if err != nil {

		return
	}
	list := resp.Hits.Hits
	var indexNames []string
	for _, v := range list {
		m := v.Source["metadata"].(map[string]interface{})
		indexNames = append(indexNames, m["index_name"].(string))

	}
	m := make(map[string]interface{})
	m["indexnames"] = indexNames
	h.WriteOKJSON(w, m)

	return
}
