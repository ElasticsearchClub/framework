/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/agent"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/host"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/proxy"
	"infini.sh/framework/core/util"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type APIHandler struct {
	api.Handler
}

func (h *APIHandler) heartbeat(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")
	sm := agent.GetStateManager()
	inst, err := sm.GetAgent(id)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	syncToES := inst.Status != "online"
	inst.Status = "online"
	hostIP := util.ClientIP(req)
	log.Tracef("heartbeat from [%s]", hostIP)
	ag, err := sm.UpdateAgent(inst, syncToES)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if syncToES {
		host.UpdateHostAgentStatus(ag.ID, "online")
	}
	taskState := map[string]map[string]string{}
	for _, cluster := range ag.Clusters {
		taskState[cluster.ClusterID] = map[string]string{
			"cluster_metric": sm.GetState(cluster.ClusterID).ClusterMetricTask.NodeUUID,
		}
	}

	h.WriteJSON(w, util.MapStr{
		"agent_id":   id,
		"success":    true,
		"task_state": taskState,
		"timestamp":  time.Now().Unix(),
	}, 200)
}

func (h *APIHandler) getIP(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	remoteHost := util.ClientIP(req)
	h.WriteJSON(w, util.MapStr{
		"ip": remoteHost,
	}, http.StatusOK)
}

const APIKeyBucket = "console-api-key"
const HTTPHeaderAPIKey = "X-API-KEY"

func (h *APIHandler) createInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	var obj = &agent.Instance{}
	err := h.DecodeJSON(req, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if obj.Port == 0 {
		h.WriteError(w, fmt.Sprintf("invalid port [%d] of agent", obj.Port), http.StatusInternalServerError)
		return
	}
	if obj.Schema == "" {
		obj.Schema = "http"
	}
	q := &orm.Query{
		Size: 2,
	}
	sm := agent.GetStateManager()
	res, err := sm.GetAgentClient().GetInstanceBasicInfo(nil, obj.GetEndpoint())
	if err != nil {
		errStr := fmt.Sprintf("get agent instance basic info error: %s", err.Error())
		h.WriteError(w,errStr , http.StatusInternalServerError)
		log.Error(errStr)
		return
	}
	if id, ok := res["id"].(string); !ok {
		errStr :=fmt.Sprintf("got unexpected response of agent instance basic info: %s", util.MustToJSON(res))
		h.WriteError(w, errStr , http.StatusInternalServerError)
		log.Error(errStr)
		return
	}else{
		obj.ID = id
	}
	if v, ok := res["name"].(string); ok {
		obj.Name = v
	}

	remoteIP := util.ClientIP(req)
	q.Conds = orm.And(orm.Eq("_id", obj.ID))
	err, result := orm.Search(obj, q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if len(result.Result) > 0 {
		errMsg := fmt.Sprintf("agent [%s] already exists", remoteIP)
		h.WriteError(w, errMsg, http.StatusInternalServerError)
		log.Error(errMsg)
		return
	}

	//match clusters
	obj.RemoteIP = remoteIP
	obj.Enrolled = false

	err = orm.Create(nil, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	log.Infof("receive agent register from host [%s]: %s", obj.RemoteIP, util.MustToJSON(obj))

	apiKey := req.Header.Get(HTTPHeaderAPIKey)
	var isValidKey bool
	if apiKey = strings.TrimSpace(apiKey); apiKey != "" {
		buf, err := kv.GetValue(APIKeyBucket, []byte(apiKey))
		if err != nil {
			h.WriteError(w, err.Error(), http.StatusInternalServerError)
			log.Error(err)
			return
		}
		if string(buf) == "1" {
			isValidKey = true
		}
	}
	if isValidKey {
		obj.Enrolled = true
		clusters, err := enrollInstance(obj.ID)
		if err != nil {
			h.WriteError(w, err.Error(), http.StatusInternalServerError)
			log.Error(err)
			return
		}
		h.WriteJSON(w, util.MapStr{
			"_id":      obj.ID,
			"clusters": clusters,
			"result":   "created",
		}, 200)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"_id":    obj.ID,
		"result": "acknowledged",
	}, 200)

}

func enrollInstance(agentID string) (map[string]interface{}, error) {
	obj := agent.Instance{}

	obj.ID = agentID
	exists, err := orm.Get(&obj)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent [%s]: %w", agentID, err)
	}
	if !exists {
		return nil, fmt.Errorf("agent [%s] not found", agentID)
	}
	clusters, err := getMatchedClusters(obj.RemoteIP, obj.Clusters)
	if err != nil {
		return nil, err
	}

	var filterClusters []agent.ESCluster
	//remove clusters of not matched
	for i, cluster := range obj.Clusters {
		if vmap, ok := clusters[cluster.ClusterName].(map[string]interface{}); ok {
			obj.Clusters[i].ClusterID = vmap["cluster_id"].(string)
			filterClusters = append(filterClusters, obj.Clusters[i])
		}
	}
	obj.Clusters = filterClusters

	log.Infof("register agent from host [%s]: %s", obj.RemoteIP, util.MustToJSON(obj))
	sm := agent.GetStateManager()
	err = sm.EnrollAgent(&obj, clusters)
	if err == nil {
		berr := bindAgentToHostByIP(&obj)
		if berr != nil {
			log.Error("auto bind agent [%s] to host [%s] error: %v", obj.ID, obj.MajorIP, berr)
		}

	}
	return clusters, err
}

func bindAgentToHostByIP(ag *agent.Instance) error{
	err, result := orm.GetBy("ip", ag.MajorIP, host.HostInfo{})
	if err != nil {
		return err
	}
	if len(result.Result) > 0 {
		buf := util.MustToJSONBytes(result.Result[0])
		hostInfo := &host.HostInfo{}
		err = util.FromJSONBytes(buf, hostInfo)
		if err != nil {
			return err
		}
		sm := agent.GetStateManager()
		if ag.Status == "" {
			_, err1 := sm.GetAgentClient().GetHostInfo(nil, ag.GetEndpoint(), ag.ID)
			if err1 == nil {
				ag.Status = "online"
			}else{
				ag.Status = "offline"
			}
		}

		hostInfo.AgentStatus = ag.Status
		hostInfo.AgentID = ag.ID
		err = orm.Update(nil, hostInfo)
		if err != nil {
			return  err
		}

		err = sm.GetAgentClient().DiscoveredHost(nil, ag.GetEndpoint(), util.MapStr{
			"host_id": hostInfo.ID,
		})
		if err != nil {
			return  err
		}
	}
	return nil
}

func (h *APIHandler) getInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")

	obj := agent.Instance{}
	obj.ID = id

	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":   id,
			"found": false,
		}, http.StatusNotFound)
		return
	}
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"found":   true,
		"_id":     id,
		"_source": obj,
	}, 200)
}

func (h *APIHandler) updateInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")
	obj := agent.Instance{}

	obj.ID = id
	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":    id,
			"result": "not_found",
		}, http.StatusNotFound)
		return
	}

	if !obj.Enrolled {
		h.WriteError(w, fmt.Sprintf("agent [%s] is not allowed to update since it is not enrolled", id), http.StatusInternalServerError)
		return
	}

	newObj := agent.Instance{}
	err = h.DecodeJSON(req, &newObj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	if newObj.Port != obj.Port {
		obj.Port = newObj.Port
	}
	if newObj.Schema != obj.Schema {
		obj.Schema = newObj.Schema
	}
	if len(newObj.Version) > 0 {
		obj.Version = newObj.Version
	}
	if len(newObj.IPS) > 0 {
		obj.IPS = newObj.IPS
	}
	newMatchedClusters, err := h.updateInstanceNodes(&obj, newObj.Clusters)

	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	log.Infof("update agent [%s]: %v", obj.RemoteIP, util.MustToJSON(obj))
	//err = orm.Update(&obj)
	//if err != nil {
	//	h.WriteError(w, err.Error(), http.StatusInternalServerError)
	//	log.Error(err)
	//	return
	//}

	sm := agent.GetStateManager()
	_, err = sm.UpdateAgent(&obj, true)
	if err != nil {
		log.Error(err)
	}
	h.WriteJSON(w, util.MapStr{
		"_id":      obj.ID,
		"result":   "updated",
		"clusters": newMatchedClusters,
	}, 200)
}
func (h *APIHandler) updateInstanceNodes(obj *agent.Instance, esClusters []agent.ESCluster) (map[string]interface{}, error) {
	if len(esClusters) == 0 {
		return nil, fmt.Errorf("clusters should not be empty")
	}

	clusters := map[string]agent.ESCluster{}
	var newClusters []agent.ESCluster
	for _, nc := range esClusters {
		if strings.TrimSpace(nc.ClusterID) == "" {
			newClusters = append(newClusters, nc)
			continue
		}
		clusters[nc.ClusterID] = nc
	}
	var toUpClusters []agent.ESCluster
	for _, cluster := range obj.Clusters {
		if upCluster, ok := clusters[cluster.ClusterID]; ok {
			newUpCluster := agent.ESCluster{
				ClusterUUID: cluster.ClusterUUID,
				ClusterName: upCluster.ClusterName,
				ClusterID:   cluster.ClusterID,
				Nodes:       upCluster.Nodes,
				Task:        cluster.Task,
			}
			toUpClusters = append(toUpClusters, newUpCluster)
			continue
		}
		//todo log delete nodes
	}
	var (
		matchedClusters map[string]interface{}
		err             error
	)
	if len(newClusters) > 0 {
		matchedClusters, err = getMatchedClusters(obj.RemoteIP, newClusters)
		if err != nil {
			return nil, err
		}
		//filter already
		//for _, cluster := range toUpClusters {
		//	if _, ok := matchedClusters[cluster.ClusterName]; ok {
		//		delete(matchedClusters, cluster.ClusterName)
		//	}
		//}
	}
	//attach old cluster
	oldMatchedClusters, err := getMatchedClusters(obj.RemoteIP, toUpClusters)
	if err != nil {
		return nil, err
	}

	for clusterName, matchedCluster := range matchedClusters {
		if vm, ok := matchedCluster.(map[string]interface{}); ok {
			cluster := agent.ESCluster{
				ClusterName: clusterName,
			}
			if v, ok := vm["cluster_uuid"].(string); ok {
				cluster.ClusterUUID = v
			}
			if v, ok := vm["cluster_id"].(string); ok {
				cluster.ClusterID = v
			}
			toUpClusters = append(toUpClusters, cluster)
		}
	}
	obj.Clusters = toUpClusters
	if matchedClusters == nil {
		matchedClusters = map[string]interface{}{}
	}
	err = util.MergeFields(matchedClusters, oldMatchedClusters, true)
	return matchedClusters, err

}
func (h *APIHandler) setTaskToInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")
	reqBody := []struct {
		ClusterID string `json:"cluster_id"`
		NodeUUID  string `json:"node_uuid"`
	}{}

	err := h.DecodeJSON(req, &reqBody)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	sm := agent.GetStateManager()
	for _, node := range reqBody {
		err = sm.SetAgentTask(node.ClusterID, id, node.NodeUUID)
		if err != nil {
			h.WriteError(w, err.Error(), http.StatusInternalServerError)
			log.Error(err)
			return
		}
	}

	h.WriteJSON(w, util.MapStr{
		"result": "success",
	}, 200)
}

func (h *APIHandler) deleteInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("instance_id")

	obj := agent.Instance{}
	obj.ID = id

	exists, err := orm.Get(&obj)
	if !exists || err != nil {
		h.WriteJSON(w, util.MapStr{
			"_id":    id,
			"result": "not_found",
		}, http.StatusNotFound)
		return
	}

	if obj.Enrolled {
		err = agent.GetStateManager().DeleteAgent(obj.ID)
		if err != nil {
			log.Error(err)
		}
	}

	err = orm.Delete(nil, &obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		log.Error(err)
		return
	}
	host.UpdateHostAgentStatus(obj.ID, "deleted")

	h.WriteJSON(w, util.MapStr{
		"_id":    obj.ID,
		"result": "deleted",
	}, 200)
}

func (h *APIHandler) getInstanceStats(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var instanceIDs = []string{}
	err := h.DecodeJSON(req, &instanceIDs)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(instanceIDs) == 0 {
		h.WriteJSON(w, util.MapStr{}, http.StatusOK)
		return
	}
	q := orm.Query{}
	queryDSL := util.MapStr{
		"query": util.MapStr{
			"terms": util.MapStr{
				"_id": instanceIDs,
			},
		},
	}
	q.RawQuery = util.MustToJSONBytes(queryDSL)

	err, res := orm.Search(&agent.Instance{}, &q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	result := util.MapStr{}
	for _, item := range res.Result {
		instance := agent.Instance{}
		buf := util.MustToJSONBytes(item)
		util.MustFromJSONBytes(buf, &instance)
		endpoint := instance.GetEndpoint()
		gid := instance.ID
		res, err := proxy.DoProxyRequest(&proxy.Request{
			Endpoint: endpoint,
			Method:   http.MethodGet,
			Path:     "/stats",
		})
		if err != nil {
			log.Error(err)
			result[gid] = util.MapStr{}
			continue
		}
		var resMap = util.MapStr{}
		err = util.FromJSONBytes(res.Body, &resMap)
		if err != nil {
			result[gid] = util.MapStr{}
			log.Errorf("get stats of %v error: %v", endpoint, err)
			continue
		}

		result[gid] = resMap
	}
	h.WriteJSON(w, result, http.StatusOK)
}

func (h *APIHandler) enrollInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var instanceIDs = []string{}
	err := h.DecodeJSON(req, &instanceIDs)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(instanceIDs) == 0 {
		h.WriteJSON(w, util.MapStr{}, http.StatusOK)
		return
	}
	q := orm.Query{}
	queryDSL := util.MapStr{
		"query": util.MapStr{
			"terms": util.MapStr{
				"_id": instanceIDs,
			},
		},
	}
	q.RawQuery = util.MustToJSONBytes(queryDSL)

	err, res := orm.Search(&agent.Instance{}, &q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	errors := util.MapStr{}
	for _, item := range res.Result {
		instance := agent.Instance{}
		buf := util.MustToJSONBytes(item)
		err = util.FromJSONBytes(buf, &instance)
		if err != nil {
			errors[instance.ID] = util.MapStr{
				"error": err.Error(),
			}
			log.Error(err)
			continue
		}
		_, err = enrollInstance(instance.ID)
		if err != nil {
			errors[instance.ID] = util.MapStr{
				"error": err.Error(),
			}
			log.Error(err)
			continue
		}
	}

	var resBody = util.MapStr{}
	if len(errors) > 0 {
		resBody["errors"] = errors
		resBody["success"] = false
	} else {
		resBody["success"] = true
	}

	h.WriteJSON(w, resBody, http.StatusOK)
}

func (h *APIHandler) searchInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	var (
		keyword = h.GetParameterOrDefault(req, "keyword", "")
		//queryDSL    = `{"query":{"bool":{"must":[%s]}}, "size": %d, "from": %d}`
		strSize      = h.GetParameterOrDefault(req, "size", "20")
		strFrom      = h.GetParameterOrDefault(req, "from", "0")
		unregistered = h.GetParameter(req, "unregistered")
	)

	var (
		mustQ       []interface{}
		enrolledVal = true
	)
	if unregistered == "1" {
		enrolledVal = false
	}
	mustQ = append(mustQ, util.MapStr{
		"term": util.MapStr{
			"enrolled": util.MapStr{
				"value": enrolledVal,
			},
		},
	})

	if keyword != "" {
		mustQ = append(mustQ, util.MapStr{
			"query_string": util.MapStr{
				"default_field": "*",
				"query":         keyword,
			},
		})
	}
	size, _ := strconv.Atoi(strSize)
	if size <= 0 {
		size = 20
	}
	from, _ := strconv.Atoi(strFrom)
	if from < 0 {
		from = 0
	}

	queryDSL := util.MapStr{
		"size": size,
		"from": from,
		"query": util.MapStr{
			"bool": util.MapStr{
				"must": mustQ,
			},
		},
	}

	q := orm.Query{}
	q.RawQuery = util.MustToJSONBytes(queryDSL)

	err, res := orm.Search(&agent.Instance{}, &q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//searchRes := elastic.SearchResponse{}
	//util.MustFromJSONBytes(res.Raw, &searchRes)
	//for _, hit := range searchRes.Hits.Hits {
	//	hit.Source["task_count"] =
	//}

	h.Write(w, res.Raw)
}

func (h *APIHandler) getClusterInstance(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	clusterID := h.GetParameterOrDefault(req, "cluster_id", "")
	if clusterID == "" {
		h.WriteError(w, "parameter cluster_id should not be empty", http.StatusInternalServerError)
		return
	}
	esClient := elastic.GetClient(clusterID)
	nodes, err := esClient.CatNodes("id,ip,name")
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	nodesM := make(map[string]*struct {
		NodeID    string
		IP        string
		Name      string
		AgentHost string
		Owner     bool
	}, len(nodes))
	for _, node := range nodes {
		nodesM[node.Id] = &struct {
			NodeID    string
			IP        string
			Name      string
			AgentHost string
			Owner     bool
		}{NodeID: node.Id, IP: node.Ip, Name: node.Name}
	}
	query := util.MapStr{
		"query": util.MapStr{
			"term": util.MapStr{
				"clusters.cluster_id": util.MapStr{
					"value": clusterID,
				},
			},
		},
	}
	q := &orm.Query{
		RawQuery: util.MustToJSONBytes(query),
	}
	err, result := orm.Search(agent.Instance{}, q)
	if err != nil {
		log.Error(err)
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, row := range result.Result {
		buf := util.MustToJSONBytes(row)
		inst := &agent.Instance{}
		util.MustFromJSONBytes(buf, inst)
		for _, cluster := range inst.Clusters {
			for _, n := range cluster.Nodes {
				if _, ok := nodesM[n.UUID]; ok {
					nodesM[n.UUID].AgentHost = inst.RemoteIP
					nodesM[n.UUID].Owner = cluster.Task.ClusterMetric.TaskNodeID == n.UUID
				}
			}
		}
	}

	h.WriteJSON(w, nodesM, 200)
}

func getMatchedClusters(host string, clusters []agent.ESCluster) (map[string]interface{}, error) {
	resultClusters := map[string]interface{}{}
	for _, cluster := range clusters {
		queryDsl := util.MapStr{
			"query": util.MapStr{
				"bool": util.MapStr{
					"should": []util.MapStr{
						//{
						//	"term": util.MapStr{
						//		"cluster_uuid": util.MapStr{
						//			"value": cluster.ClusterUUID,
						//		},
						//	},
						//},
						{
							"bool": util.MapStr{
								"minimum_should_match": 1,
								//"must": []util.MapStr{
								//	{
								//		"prefix": util.MapStr{
								//			"host": util.MapStr{
								//				"value": host,
								//			},
								//		},
								//	},
								//},
								"should": []util.MapStr{
									{
										"term": util.MapStr{
											"raw_name": util.MapStr{
												"value": cluster.ClusterName,
											},
										},
									},
									{
										"term": util.MapStr{
											"name": util.MapStr{
												"value": cluster.ClusterName,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		q := &orm.Query{
			RawQuery: util.MustToJSONBytes(queryDsl),
		}
		log.Trace("match query dsl: ", string(q.RawQuery))
		err, result := orm.Search(elastic.ElasticsearchConfig{}, q)
		if err != nil {
			return nil, err
		}
		if len(result.Result) == 1 {
			buf := util.MustToJSONBytes(result.Result[0])
			esConfig := elastic.ElasticsearchConfig{}
			util.MustFromJSONBytes(buf, &esConfig)
			resultClusters[cluster.ClusterName] = map[string]interface{}{
				"cluster_id":   esConfig.ID,
				"cluster_uuid": esConfig.ClusterUUID,
				"basic_auth":   esConfig.BasicAuth,
			}
		}
	}
	return resultClusters, nil
}

func (h *APIHandler) getClusterAuth(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	clusterID := h.GetParameterOrDefault(req, "cluster_id", "")
	if clusterID == "" {
		h.WriteError(w, "parameter cluster_id should not be empty", http.StatusInternalServerError)
		return
	}
	//esClient := elastic.GetClient(clusterID)
}

func getAgentByHost(host string) (*agent.Instance, error) {
	q := &orm.Query{
		Size: 1,
	}
	q.Conds = orm.And(orm.Eq("remote_ip", host))
	inst := agent.Instance{}
	err, result := orm.Search(inst, q)
	if err != nil {
		return nil, err
	}
	if len(result.Result) == 0 {
		return nil, nil
	}
	buf, err := util.ToJSONBytes(result.Result[0])
	if err != nil {
		return nil, err
	}
	err = util.FromJSONBytes(buf, &inst)
	return &inst, err
}