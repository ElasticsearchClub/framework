/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/api/rbac"
	httprouter "infini.sh/framework/core/api/router"
	"net/http"
)

func (h APIHandler) Profile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	reqUser, err := rbac.FromUserContext(r.Context())
	if err != nil {
		h.ErrorInternalServer(w, err.Error())
		return
	}

	user, err := h.User.Get(reqUser.UserId)
	if err != nil {
		h.WriteJSON(w, api.NotFoundResponse(reqUser.UserId), 404)
		return
	}

	////handle tenant
	////only for native realm provider
	//if user.Tenant != nil {
	//	u["tenant"] = user.Tenant
	//}

	user.Payload = nil
	user.Password = ""
	h.WriteOKJSON(w, api.FoundResponse(reqUser.UserId, user))

}
