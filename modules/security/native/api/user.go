/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package api

import (
	"bytes"
	"errors"
	"github.com/buger/jsonparser"
	log "github.com/cihub/seelog"
	"golang.org/x/crypto/bcrypt"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic"
	"net/http"
	"sort"
	"time"
)

func (h APIHandler) CreateUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var user rbac.User
	err := h.DecodeJSON(r, &user)
	if err != nil {
		h.Error400(w, err.Error())
		return
	}
	if user.Name == ""  {
		h.Error400(w, "username is require")
		return
	}
	//localUser, err := biz.FromUserContext(r.Context())
	//if err != nil {
	//	log.Error(err.Error())
	//	h.ErrorInternalServer(w, err.Error())
	//	return
	//}
	if h.userNameExists(w, user.Name) {
		return
	}
	randStr := util.GenerateRandomString(8)
	hash, err := bcrypt.GenerateFromPassword([]byte(randStr), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	user.Password = string(hash)

	now:=time.Now()
	user.Created = &now
	user.Updated = &now

	id, err := h.User.Create(&user)
	user.ID = id
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	_ = h.WriteOKJSON(w, util.MapStr{
		"_id":      id,
		"password": randStr,
		"result":   "created",
	})
	return

}

func (h APIHandler) userNameExists(w http.ResponseWriter, name string) bool {
	u, err := h.User.GetBy("name", name)
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return true
	}
	if u != nil {
		h.ErrorInternalServer(w, "user name already exists")
		return true
	}
	return false
}

func (h APIHandler) GetUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	user, err := h.User.Get(id)
	if errors.Is(err, elastic.ErrNotFound) {
		h.WriteJSON(w, api.NotFoundResponse(id), http.StatusNotFound)
		return
	}

	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	h.WriteOKJSON(w, api.FoundResponse(id, user))
	return
}

func (h APIHandler) UpdateUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	var user rbac.User
	err := h.DecodeJSON(r, &user)
	if err != nil {
		_ = log.Error(err.Error())
		h.Error400(w, err.Error())
		return
	}
	//localUser, err := biz.FromUserContext(r.Context())
	//if err != nil {
	//	log.Error(err.Error())
	//	h.ErrorInternalServer(w, err.Error())
	//	return
	//}
	oldUser, err := h.User.Get(id)
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	if user.Name != oldUser.Name && h.userNameExists(w, user.Name) {
		return
	}

	now:=time.Now()
	user.Updated = &now
	user.Created = oldUser.Created
	user.ID = id
	err = h.User.Update(&user)

	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	//let user relogin after roles changed
	sort.Slice(user.Roles, func(i, j int) bool {
		return user.Roles[i].ID < user.Roles[j].ID
	})
	sort.Slice(oldUser.Roles, func(i, j int) bool {
		return oldUser.Roles[i].ID < oldUser.Roles[j].ID
	})
	changeLog, _ := util.DiffTwoObject(user.Roles, oldUser.Roles)
	if len(changeLog) > 0 {
		rbac.DeleteUserToken(id)
	}
	_ = h.WriteOKJSON(w, api.UpdateResponse(id))
	return
}


func (h APIHandler) DeleteUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	//localUser, err := biz.FromUserContext(r.Context())
	//if err != nil {
	//	log.Error(err.Error())
	//	h.ErrorInternalServer(w, err.Error())
	//	return
	//}
	err := h.User.Delete(id)
	if errors.Is(err, elastic.ErrNotFound) {
		h.WriteJSON(w, api.NotFoundResponse(id), http.StatusNotFound)
		return
	}
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	rbac.DeleteUserToken(id)
	_ = h.WriteOKJSON(w, api.DeleteResponse(id))
	return
}

func (h APIHandler) SearchUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var (
		keyword = h.GetParameterOrDefault(r, "keyword", "")
		from    = h.GetIntOrDefault(r, "from", 0)
		size    = h.GetIntOrDefault(r, "size", 20)
	)

	res, err := h.User.Search(keyword, from, size)
	if err != nil {
		log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	//remove password field
	hitsBuf := bytes.Buffer{}
	hitsBuf.Write([]byte("["))
	jsonparser.ArrayEach(res.Raw, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		value = jsonparser.Delete(value, "_source", "password")
		hitsBuf.Write(value)
		hitsBuf.Write([]byte(","))
	}, "hits", "hits")
	buf := hitsBuf.Bytes()
	if buf[len(buf)-1] == ',' {
		buf[len(buf)-1] = ']'
	}else{
		hitsBuf.Write([]byte("]"))
	}
	res.Raw, err = jsonparser.Set(res.Raw, hitsBuf.Bytes(), "hits", "hits")
	if err != nil {
		log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}

	h.Write(w, res.Raw)
	return

}
func (h APIHandler) UpdateUserPassword(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	var req = struct {
		Password string `json:"password"`
	}{}
	err := h.DecodeJSON(r, &req)
	if err != nil {
		_ = log.Error(err.Error())
		h.Error400(w, err.Error())
		return
	}
	//localUser, err := biz.FromUserContext(r.Context())
	//if err != nil {
	//	log.Error(err.Error())
	//	h.ErrorInternalServer(w, err.Error())
	//	return
	//}
	user, err := h.User.Get(id)
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	user.Password = string(hash)
	//t:=time.Now()
	//user.Updated =&t
	err = h.User.Update(&user)
	if err != nil {
		_ = log.Error(err.Error())
		h.ErrorInternalServer(w, err.Error())
		return
	}
	//disable old token to let user login
	rbac.DeleteUserToken(id)

	_ = h.WriteOKJSON(w, api.UpdateResponse(id))
	return

}