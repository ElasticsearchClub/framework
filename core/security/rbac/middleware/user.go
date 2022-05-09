package middleware

import (
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/security/rbac"
	"infini.sh/framework/core/util"
	"net/http"
)

func LoginRequired(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		claims, err := ValidateLogin(r.Header.Get("Authorization"))
		if err != nil {
			w = handleError(w, http.StatusUnauthorized, err)
			return
		}
		r = r.WithContext(rbac.NewUserContext(r.Context(), claims))
		h(w, r, ps)
	}
}

func PermissionRequired(h httprouter.Handle, permissions ...string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		claims, err := ValidateLogin(r.Header.Get("Authorization"))
		if err != nil {

			w = handleError(w, http.StatusUnauthorized, err)
			return
		}
		err = ValidatePermission(claims, permissions)
		if err != nil {
			w = handleError(w, http.StatusForbidden, err)
			return
		}
		r = r.WithContext(rbac.NewUserContext(r.Context(), claims))
		h(w, r, ps)
	}
}
func handleError(w http.ResponseWriter, statusCode int, err error) http.ResponseWriter {
	w.Header().Set("Content-type", util.ContentTypeJson)
	w.WriteHeader(statusCode)
	json := util.ToJson(util.MapStr{
		"error": util.MapStr{
			"status": statusCode,
			"reason": err.Error(),
		},
	}, true)
	w.Write([]byte(json))

	return w
}
