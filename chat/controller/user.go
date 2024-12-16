package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/filinvadim/warpnet/chat/entity"
	"github.com/filinvadim/warpnet/chat/service"
	"github.com/julienschmidt/httprouter"
)

func (c *Controller) GetUsers(svc service.GetUsers) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		ctx := r.Context()
		userID, _ := ctx.Value(entity.ContextKeyUserID).(string)
		id, err := strconv.Atoi(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req := service.GetUsersRequest{
			UserID: id,
		}
		res, err := svc(ctx, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(res)
	}
}
