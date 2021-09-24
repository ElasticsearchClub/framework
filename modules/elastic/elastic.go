/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package elastic

import (
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/elastic/model"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/api"
	. "infini.sh/framework/modules/elastic/common"
	"strings"
	"time"
)

func (module ElasticModule) Name() string {
	return "Elastic"
}

var (
	defaultConfig = ModuleConfig{
		Elasticsearch:       "default",
		RemoteConfigEnabled: false,
		HealthCheckConfig: HealthCheckConfig{
			Enabled:  true,
			Interval: "10s",
		},
		MonitoringConfig: MonitoringConfig{
			Enabled:  false,
			Interval: "10s",
		},
		ORMConfig: ORMConfig{
			Enabled:      false,
			InitTemplate: true,
			IndexPrefix:  ".infini-",
		},
		StoreConfig: StoreConfig{
			Enabled: false,
		},
	}
)

func getDefaultConfig() ModuleConfig {
	return defaultConfig
}

var m = map[string]model.ElasticsearchConfig{}

func loadFileBasedElasticConfig() {
	var configs []model.ElasticsearchConfig
	exist, err := env.ParseConfig("elasticsearch", &configs)
	if exist && err != nil {
		panic(err)
	}

	if exist {
		for _, v := range configs {
			if !v.Enabled {
				log.Debug("elasticsearch ", v.Name, " is not enabled")
				continue
			}
			v.Source = "file"
			if v.ID == "" {
				if v.Name == "" {
					panic(errors.Errorf("invalid elasticsearch config, %v", v))
				}
				v.ID = v.Name
			}
			m[v.ID] = v
		}
	}
}

func loadESBasedElasticConfig() {
	configs := []model.ElasticsearchConfig{}
	query := elastic.SearchRequest{From: 0, Size: 1000} //TODO handle clusters beyond 1000
	result, err := elastic.GetClient(moduleConfig.Elasticsearch).Search(orm.GetIndexName(model.ElasticsearchConfig{}), &query)
	if err != nil {
		log.Error(err)
		return
	}

	if len(result.Hits.Hits) > 0 {
		for _, v1 := range result.Hits.Hits {
			cfg := model.ElasticsearchConfig{}
			bytes := util.MustToJSONBytes(v1.Source)
			util.MustFromJSONBytes(bytes, &cfg)
			cfg.ID = v1.ID.(string)
			cfg.Discovery.Enabled = true
			configs = append(configs, cfg)
		}
	}

	for _, v := range configs {
		if !v.Enabled {
			log.Debug("elasticsearch ", v.Name, " is not enabled")
			continue
		}
		v.Source = "elastic"
		if v.ID == "" {
			if v.Name == "" {
				log.Errorf("invalid elasticsearch config, %v", v)
				continue
			}
			v.ID = v.Name
		}
		m[v.ID] = v
	}

}


func initElasticInstances() {
	for k, esConfig := range m {
		if !esConfig.Enabled {
			log.Warn("elasticsearch ", esConfig.Name, " is not enabled")
			continue
		}
		client, err := InitClientWithConfig(esConfig)
		if err != nil {
			log.Error("elasticsearch ", esConfig.Name, err)
			continue
		}
		elastic.RegisterInstance(k, esConfig, client)
	}
}

var moduleConfig = ModuleConfig{}

func (module ElasticModule) Setup(cfg *config.Config) {

	loadFileBasedElasticConfig()

	initElasticInstances()

	moduleConfig = getDefaultConfig()

	exists,err:=env.ParseConfig("elastic", &moduleConfig)

	if exists&&err != nil {
		panic(err)
	}

	if moduleConfig.ORMConfig.Enabled {
		client := elastic.GetClient(moduleConfig.Elasticsearch)
		if moduleConfig.ORMConfig.InitTemplate {
			client.InitDefaultTemplate(moduleConfig.ORMConfig.TemplateName, moduleConfig.ORMConfig.IndexPrefix)
		}
		handler := ElasticORM{Client: client, Config: moduleConfig.ORMConfig}
		orm.Register("elastic", handler)

		err = orm.RegisterSchemaWithIndexName(model.ElasticsearchConfig{}, "cluster")
		if err != nil {
			panic(err)
		}

		err = orm.RegisterSchemaWithIndexName(MonitoringItem{}, "monitoring")
		if err != nil {
			panic(err)
		}
	}

	if moduleConfig.StoreConfig.Enabled {
		client := elastic.GetClient(moduleConfig.Elasticsearch)
		handler := ElasticStore{Client: client, Config: moduleConfig.StoreConfig}
		kv.Register("elastic", handler)
	}

	api.Init(moduleConfig)
}

func (module ElasticModule) Stop() error {
	return nil
}

func monitoring() {
	task1 := task.ScheduleTask{
		Description: "monitoring for elasticsearch clusters",
		Type:        "interval",
		Interval:    "10s",
		Task: func() {
			elastic.WalkMetadata(func(key, value interface{}) bool {
				k:=key.(string)
				if value==nil{
					return false
				}

				v,ok:=value.(*model.ElasticsearchMetadata)
				if ok{
					if !v.Config.Monitored || !v.Config.Enabled {
						return false
					}

					log.Tracef("run monitoring task for elasticsearch: " + k)
					client := elastic.GetClient(k)
					stats := client.GetClusterStats()
					indexStats,err := client.GetStats()
					if err != nil {
						log.Error(v.Config.Name, " get cluster stats error: ", err)
						return false
					}

					v.ReportSuccess()

					item := MonitoringItem{}
					item.Elasticsearch = v.Config.ID
					item.ClusterStats = stats
					if indexStats!=nil{

						//replace . to _dot_
						for k,v:=range indexStats.Indices{
							if util.PrefixStr(k,"."){
								delete(indexStats.Indices,k)
								indexStats.Indices[strings.Replace(k,".","_dot_",1)]=v
							}
						}

						item.IndexStats = indexStats
					}
					item.Timestamp = time.Now()
					item.Agent = global.Env().SystemConfig.NodeConfig
					monitoringClient:=elastic.GetClient(moduleConfig.Elasticsearch)
					_, err = monitoringClient.Index(orm.GetIndexName(item), "", "", item)
					if err != nil {
						log.Error(err)
					}
				}
				return true
			})
		},
	}

	task.RegisterScheduleTask(task1)


}

func healthCheck() {

	task2:=task.ScheduleTask{
		Description: "check for elasticsearch node availability",
		Type:        "interval",
		Interval:    "10s",
		Task: func() {
			elastic.WalkHosts(func(key, value interface{}) bool {
				k:=key.(string)

				if value==nil{
					return false
				}

				v,ok:=value.(*model.NodeAvailable)
				if ok{

					log.Debugf("check availability for node: " + k)
						avail:=util.TestTCPAddress(k)
						if avail{
							v.LastActive=time.Now()
						}else{
							v.LastFailure=time.Now()
						}
						v.Available=avail
						log.Debugf("node [%v], connection available: [%v]" ,k,v.Available)
				}
				return true
			})
		},
	}
	task.RegisterScheduleTask(task2)
}

func discovery() {
	discoveryMetadata(false)
}

func discoveryMetadata(force bool) {
	elastic.WalkConfigs(func(key, value interface{}) bool {
		cfg,ok:=value.(*model.ElasticsearchConfig)
		if ok&&cfg!=nil{
			go func(cfg *model.ElasticsearchConfig) {
				//fmt.Println("checking:",cfg.Name,"Discovery:",cfg.Discovery.Enabled)
				if cfg.Enabled||force {
					oldMetadata := elastic.GetOrInitMetadata(cfg)
					client := elastic.GetClient(cfg.ID)

					//TODO 按需加载节点信息，检测节点连通性
					nodes, err := client.GetNodes()

					if err != nil {
						log.Debug(cfg.Name," ",err)
						oldMetadata.ReportFailure()
						return
					}

					if nodes == nil || len(*nodes) <= 0 {
						log.Errorf("elasticsearch [%v] failed to get nodes info",cfg.Name)
						oldMetadata.ReportFailure()
						return
					}

					oldMetadata.ReportSuccess()

					newMetadata := model.ElasticsearchMetadata{Config: cfg}
					newMetadata.Init(true)

					var nodesChanged = false

					if cfg.Discovery.Enabled{

						//Nodes
						//if util.ContainsAnyInArray("nodes", cfg.Discovery.Modules) {
						var oldNodesTopologyVersion = 0
						if oldMetadata == nil {
							nodesChanged = true
						} else {
							oldNodesTopologyVersion = oldMetadata.NodesTopologyVersion
							newMetadata.NodesTopologyVersion = oldNodesTopologyVersion
							newMetadata.Nodes = oldMetadata.Nodes

							if len(*nodes) != len(oldMetadata.Nodes) {
								nodesChanged = true
							} else {
								for k, v := range *nodes {

									elastic.GetOrInitHost(v.Http.PublishAddress)

									v1, ok := oldMetadata.Nodes[k]
									if ok {
										if v.Http.PublishAddress != v1.Http.PublishAddress {
											nodesChanged = true
										}
									} else {
										nodesChanged = true
										break
									}
								}
							}
						}

						if nodesChanged {
							newMetadata.NodesTopologyVersion = oldNodesTopologyVersion + 1
							newMetadata.Nodes = *nodes
						}

					}

					//Indices
					var indicesChanged bool
					indices, err := client.GetIndices("")
					if err != nil {
						log.Error(err)
						return
					}
					if indices != nil {
						//TODO check if that changed or skip replace
						newMetadata.Indices = *indices
						indicesChanged = true
					}

					//Shards
					var shardsChanged bool
					shards, err := client.GetPrimaryShards()
					if err != nil {
						log.Error(err)
						return
					}
					if shards != nil {
						//TODO check if that changed or skip replace
						newMetadata.PrimaryShards = *shards
						shardsChanged = true
					}

					//Indices
					var aliasesChanged bool
					aliases, err := client.GetAliases()
					if err != nil {
						log.Error(err)
						return
					}
					if aliases != nil {
						//TODO check if that changed or skip replace
						newMetadata.Aliases = *aliases
						aliasesChanged = true
					}

					//health status
					var healthChanged bool
					health := client.ClusterHealth()
					if health != nil {
						//TODO check if that changed or skip replace
						newMetadata.HealthStatus = health.Status
						healthChanged = true
					}

					if nodesChanged || indicesChanged || shardsChanged || aliasesChanged || healthChanged{
						log.Debug("elasticsearch metadata updated,", len(newMetadata.Nodes),nodesChanged , indicesChanged , shardsChanged, aliasesChanged, healthChanged)
						elastic.SetMetadata(cfg.ID, &newMetadata)
					}
				}
			}(cfg)
		}
		return true
	})
}

func (module ElasticModule) Start() error {

	if moduleConfig.RemoteConfigEnabled {
		loadESBasedElasticConfig()
	}

	initElasticInstances()
	log.Trace("loadESBasedElasticConfig completed")

	t := task.ScheduleTask{
		Description: "discovery nodes topology",
		Type:        "interval",
		Interval:    "10s",
		Task:        discovery,
	}

	task.RegisterScheduleTask(t)

	discoveryMetadata(true)

	if moduleConfig.MonitoringConfig.Enabled {
		monitoring()
	}

	if moduleConfig.HealthCheckConfig.Enabled{
		healthCheck()
	}

	return nil

}

type ElasticModule struct {
}
