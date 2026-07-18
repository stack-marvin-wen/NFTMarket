package v1

import (
	"encoding/json"

	"github.com/ProjectsTask/EasySwapBase/errcode"
	"github.com/ProjectsTask/EasySwapBase/xhttp"
	"github.com/gin-gonic/gin"

	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/service/v1"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

// @Summary Get multi-chain activities
// @Tags Activity
// @Produce json
// @Param filters query string true "Activity filter JSON string"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /activities [get]
func ActivityMultiChainHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		filterParam := c.Query("filters")
		if filterParam == "" {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		var filter types.ActivityMultiChainFilterParams
		err := json.Unmarshal([]byte(filterParam), &filter)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		// filter params not include `filter_ids`, return all chain data
		if filter.ChainID == nil || len(filter.ChainID) == 0 {
			res, err := service.GetAllChainActivities(c.Request.Context(), svcCtx, filter.CollectionAddresses, filter.TokenID, filter.UserAddresses,
				filter.EventTypes, filter.Page, filter.PageSize)
			if err != nil {
				xhttp.Error(c, errcode.NewCustomErr("Get multi-chain activities failed."))
				return
			}
			xhttp.OkJson(c, res)
		} else { // return filtered data
			var chainName []string
			for _, id := range filter.ChainID {
				chainName = append(chainName, chainIDToChain[id])
			}

			res, err := service.GetMultiChainActivities(c.Request.Context(), svcCtx, filter.ChainID, chainName, filter.CollectionAddresses, filter.TokenID, filter.UserAddresses,
				filter.EventTypes, filter.Page, filter.PageSize)
			if err != nil {
				xhttp.Error(c, errcode.NewCustomErr("Get multi-chain activities failed."))
				return
			}
			xhttp.OkJson(c, res)
		}
	}
}
