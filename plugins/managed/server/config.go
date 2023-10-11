/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

import (
	log "github.com/cihub/seelog"
	httprouter "infini.sh/framework/core/api/router"
	config3 "infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/util"
	"infini.sh/framework/plugins/managed/common"
	"infini.sh/framework/plugins/managed/config"
	"net/http"
	"path"
)

func refreshConfigsRepo() {
	//load config settings from file
	if global.Env().SystemConfig.Configs.ManagerConfig.LocalConfigsRepoPath != "" {
		configRepo = common.ConfigRepo{}
		cfgPath := path.Join(global.Env().SystemConfig.Configs.ManagerConfig.LocalConfigsRepoPath, "/settings.yml")
		setCfg, err := config3.LoadFiles(cfgPath)
		if err != nil {
			panic(err)
		}

		err = setCfg.Unpack(&configRepo)
		log.Debug("loading config_repo: ", configRepo)
		if err != nil {
			panic(err)
		}

		if configRepo.InstanceGroups != nil {
			for _, v := range configRepo.InstanceGroups {
				cfgs := []string{}
				for _, f := range v.ConfigGroups {
					cfg, ok := configRepo.ConfigGroups[f]
					if ok {
						cfgs = append(cfgs, cfg.Files...)
					}
				}

				secrets := []common.Secrets{}
				for _, f := range v.Secrets {
					secret, ok := configRepo.SecretGroups[f]
					if ok {
						secrets = append(secrets, secret)
					}
				}
				for _, x := range v.Instances {
					instanceConfigFiles[x] = cfgs
					instanceSecrets[x] = secrets
				}
			}
		}
	}
}

func getSecretsForInstance(instance model.Instance) *common.Secrets {
	secrets := common.Secrets{}
	secrets.Keystore = map[string]common.KeystoreValue{}

	//get config files for static settings
	serverInit.Do(func() {
		refreshConfigsRepo()
	})

	if instanceSecrets != nil {
		v, ok := instanceSecrets[instance.ID]
		if ok {
			for _, f := range v {
				if ok {
					for m, n := range f.Keystore {
						secrets.Keystore[m] = n
					}
				}
			}
		}
	}
	return &secrets
}

func getConfigsForInstance(instance model.Instance) map[string]common.ConfigFile {
	result := map[string]common.ConfigFile{}

	//get config files for static settings
	serverInit.Do(func() {
		refreshConfigsRepo()
	})

	if instanceConfigFiles != nil {
		v, ok := instanceConfigFiles[instance.ID]
		if ok {
			for _, x := range v {
				file := path.Join(global.Env().SystemConfig.Configs.ManagerConfig.LocalConfigsRepoPath, x)
				log.Debug("prepare config:", file)
				cfg, err := config.GetConfigFromFile(global.Env().SystemConfig.Configs.ManagerConfig.LocalConfigsRepoPath, file)
				if err != nil {
					panic(err)
				}
				if cfg != nil {
					cfg.Managed = true
					result[cfg.Name] = *cfg
				}
			}
		}
	}

	//get config files from remote db

	return result
}

func (h APIHandler) refreshConfigsRepo(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	refreshConfigsRepo()
	h.WriteAckOKJSON(w)
}

func (h APIHandler) syncConfigs(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	var obj = &common.ConfigSyncRequest{}
	err := h.DecodeJSON(req, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
	}

	if global.Env().IsDebug {
		log.Trace("request:", util.MustToJSON(obj))
	}

	var res = common.ConfigSyncResponse{}
	res.Configs.CreatedConfigs = map[string]common.ConfigFile{}
	res.Configs.UpdatedConfigs = map[string]common.ConfigFile{}
	res.Configs.DeletedConfigs = map[string]common.ConfigFile{}

	//check if client is enrolled

	//check if client was marked as deleted

	//find out the client belongs to which config group

	//if server's hash didn't change, skip

	//if client's hash don't change, skip

	//find out different configs, add or delete configs
	cfgs := getConfigsForInstance(obj.Client)

	if global.Env().IsDebug {
		log.Debugf("get configs for agent(%v): %v", obj.Client.ID, util.MustToJSON(cfgs))
	}

	if cfgs == nil || len(cfgs) == 0 {

		if len(obj.Configs.Configs) > 0 {
			//set everything is deleted
			log.Debugf("no config get from manager, exists config should be all deleted for instance: %v", obj.Client.ID)
			res.Configs.DeletedConfigs = obj.Configs.Configs
			res.Changed = true
		} else {
			log.Debugf("no config found from manager for instance: %v", obj.Client.ID)
			res.Changed = false
		}
		h.WriteJSON(w, res, 200)
		return
	}

	//get configs from repo, let's diff and send to client
	if len(cfgs) > 0 {

		//find out which config content was changed, replace to new content
		if len(obj.Configs.Configs) > 0 {
			//check diff
			for k, v := range cfgs {
				x, ok := obj.Configs.Configs[k]
				//both exists
				if ok {
					if global.Env().IsDebug {
						log.Trace("both exists: ", k, ", checking version: ", v.Version, " vs ", x.Version)
					}
					//let's diff the version
					if v.Version > x.Version {
						if global.Env().IsDebug {
							log.Trace("get newly version from server, let's sync to client: ", k)
						}

						res.Configs.UpdatedConfigs[k] = v
						res.Changed = true
					} else {
						//this config no need to update
						if global.Env().IsDebug {
							log.Trace("config not changed: ", k)
						}
					}
				} else {
					if global.Env().IsDebug {
						log.Trace("found new configs: ", k, ", version: ", v.Version)
					}
					res.Configs.CreatedConfigs[k] = v
					res.Changed = true
				}
			}

			//check removed files
			for k, v := range obj.Configs.Configs {
				_, ok := cfgs[k]
				if !ok {
					//missing in server's config
					res.Configs.DeletedConfigs[k] = v
					res.Changed = true
					if global.Env().IsDebug {
						log.Trace("config was removed from server, let's mark it as deleted: ", k)
					}
				}
			}
		} else {
			if global.Env().IsDebug {
				log.Tracef("found %v new configs", len(cfgs))
			}
			res.Changed = true
			res.Configs.CreatedConfigs = cfgs
		}
	}

	//only if config changed, we change try to update the client's secrets, //TODO maybe there are coupled
	if res.Changed {
		secrets := getSecretsForInstance(obj.Client)
		res.Secrets = secrets
	}

	h.WriteJSON(w, res, 200)

}
