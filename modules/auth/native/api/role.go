/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	log "github.com/cihub/seelog"
	"github.com/mitchellh/mapstructure"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac"
	"infini.sh/framework/core/api/rbac/enum"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
	"net/http"
	"time"
)

func (h APIHandler) CreateRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	roleType := ps.MustGetParameter("type")

	//localUser, err := rbac.FromUserContext(r.Context())
	//if err != nil {
	//	log.Error(err.Error())
	//	h.ErrorInternalServer(w, err.Error())
	//	return
	//}
	err :=  rbac.IsAllowRoleType(roleType)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}
	role := &rbac.Role{
		Type: roleType,
	}
	err = h.DecodeJSON(r, role)
	if err != nil {
		h.Error400(w, err.Error())
		return
	}
	if _, ok := rbac.RoleMap[role.Name]; ok {
		h.ErrorInternalServer(w, "role name already exists")
		return
	}

	role.Created = time.Now()
	role.Updated = role.Created
	role.Type = roleType
	var id string
	id, err = h.Adapter.Role.Create(role)

	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	rbac.RoleMap[role.Name] = *role
	_ = h.WriteOKJSON(w, api.CreateResponse(id))
	return

}

func (h APIHandler) SearchRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	var (
		keyword = h.GetParameterOrDefault(r, "keyword", "")
		from    = h.GetIntOrDefault(r, "from", 0)
		size    = h.GetIntOrDefault(r, "size", 20)
	)

	res, err := h.Adapter.Role.Search(keyword, from, size)
	if err != nil {
		log.Error(err)
		h.ErrorInternalServer(w, err.Error())
		return
	}
	response := elastic.SearchResponse{}
	util.FromJSONBytes(res.Raw, &response)

	hits := response.Hits.Hits
	list := make([]elastic.IndexDocument, 0)
	total := response.GetTotal()
	var index string
	for _, v := range hits {
		index = v.Index
	}
	for k, v := range enum.BuildRoles {
		list = append(list, elastic.IndexDocument{
			ID:     k,
			Index:  index,
			Type:   "_doc",
			Source: v,
		})
		total++
	}
	list = append(list, hits...)
	response.Hits.Hits = list
	response.Hits.Total = total

	h.WriteOKJSON(w, response)
	return

}

func (h APIHandler) GetRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	role, err := h.Adapter.Role.Get(id)

	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	h.WriteOKJSON(w, api.Response{Hit: role})
	return
}

func (h APIHandler) DeleteRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")

	//localUser, err := biz.FromUserContext(r.Context())
	//if err != nil {
	//	log.Error(err.Error())
	//	h.ErrorInternalServer(w, err.Error())
	//	return
	//}
	oldRole, err := h.Role.Get(id)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
	}
	err = h.Adapter.Role.Delete(id)

	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	delete(rbac.RoleMap, oldRole.Name)
	_ = h.WriteOKJSON(w, api.DeleteResponse(id))
	return
}

func (h APIHandler) UpdateRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	//localUser, err := biz.FromUserContext(r.Context())
	//if err != nil {
	//	log.Error(err.Error())
	//	h.ErrorInternalServer(w, err.Error())
	//	return
	//}
	role := &rbac.Role{}
	err := h.DecodeJSON(r, role)
	if err != nil {
		h.Error400(w, err.Error())
		return
	}
	role.ID = id


	oldRole, err := h.Role.Get(id)
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
	}
	role.Type = oldRole.Type
	role.Updated = time.Now()
	role.Created = oldRole.Created
	err = h.Role.Update(role)
	rbac.RoleMap[role.Name] = *role

	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	_ = h.WriteOKJSON(w, api.UpdateResponse(id))
	return
}

func (h APIHandler) loadRolePermission() {
	log.Trace("start loading roles from adapter")
	rbac.RoleMap = make(map[string]rbac.Role)

	rbac.RoleMap[roleAdminName] = rbac.Role{
		Privilege: rbac.RolePrivilege{
			Platform: enum.AdminPrivilege,
		},
	}

	res, err := h.Role.Search("", 0, 1000)
	if err != nil {
		log.Error(err)
		return
	}
	response := elastic.SearchResponse{}
	util.FromJSONBytes(res.Raw, &response)

	for _, v := range response.Hits.Hits {
		var role rbac.Role
		delete(v.Source, "created")
		delete(v.Source, "updated")
		err = mapstructure.Decode(v.Source, &role)
		if err != nil {
			log.Error(err)
			return
		}
		rbac.RoleMap[role.Name] = role
	}

}
