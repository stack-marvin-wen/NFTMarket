package v1

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/ProjectsTask/EasySwapBase/errcode"
	"github.com/ProjectsTask/EasySwapBase/logger/xzap"
	"github.com/ProjectsTask/EasySwapBase/xhttp"

	"github.com/ProjectsTask/EasySwapBackend/src/api/middleware"
	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/service/v1"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

// @Summary Get collection items
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param filters query string true "Collection items filter JSON string"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/items [get]
func CollectionItemsHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		filterParam := c.Query("filters")
		if filterParam == "" {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		var filter types.CollectionItemFilterParams
		err := json.Unmarshal([]byte(filterParam), &filter)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chain, ok := chainIDToChain[filter.ChainID]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}
		res, err := service.GetItems(c.Request.Context(), svcCtx, chain, filter, collectionAddr)
		if err != nil {
			xhttp.Error(c, errcode.ErrUnexpected)
			return
		}
		xhttp.OkJson(c, res)
	}
}

// @Summary Get collection bids
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param filters query string true "Collection bids filter JSON string"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/bids [get]
func CollectionBidsHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		filterParam := c.Query("filters")
		if filterParam == "" {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		var filter types.CollectionBidFilterParams
		err := json.Unmarshal([]byte(filterParam), &filter)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chain, ok := chainIDToChain[int(filter.ChainID)]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		res, err := service.GetBids(c.Request.Context(), svcCtx, chain, collectionAddr, filter.Page, filter.PageSize)
		if err != nil {
			xhttp.Error(c, errcode.ErrUnexpected)
			return
		}
		xhttp.OkJson(c, res)
	}
}

// @Summary Get item bids in collection
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param token_id path string true "Token ID"
// @Param filters query string true "Item bids filter JSON string"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/{token_id}/bids [get]
func CollectionItemBidsHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		filterParam := c.Query("filters")
		if filterParam == "" {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		var filter types.CollectionBidFilterParams
		err := json.Unmarshal([]byte(filterParam), &filter)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		tokenID := c.Params.ByName("token_id")
		if tokenID == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chain, ok := chainIDToChain[int(filter.ChainID)]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		res, err := service.GetItemBidsInfo(c.Request.Context(), svcCtx, chain, collectionAddr, tokenID, filter.Page, filter.PageSize)
		if err != nil {
			xhttp.Error(c, errcode.ErrUnexpected)
			return
		}
		xhttp.OkJson(c, res)
	}
}

// @Summary Get item detail
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param token_id path string true "Token ID"
// @Param chain_id query int true "Chain ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/{token_id} [get]
func ItemDetailHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		tokenID := c.Params.ByName("token_id")
		if tokenID == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chainID, err := strconv.ParseInt(c.Query("chain_id"), 10, 64)
		if err != nil {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chain, ok := chainIDToChain[int(chainID)]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		res, err := service.GetItem(c.Request.Context(), svcCtx, chain, int(chainID), collectionAddr, tokenID)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("get item error"))
			return

		}
		xhttp.OkJson(c, res)
	}
}

// @Summary Get top trait prices
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param filters query string true "Top trait filter JSON string"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/top-trait [get]
func ItemTopTraitPriceHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		filterParam := c.Query("filters")
		if filterParam == "" {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		var filter types.TopTraitFilterParams
		err := json.Unmarshal([]byte(filterParam), &filter)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		chain, ok := chainIDToChain[filter.ChainID]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		res, err := service.GetItemTopTraitPrice(c.Request.Context(), svcCtx, chain, collectionAddr, filter.TokenIds)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("get item error"))
			return
		}
		xhttp.OkJson(c, res)
	}
}

// @Summary Get history sales
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param chain_id query int true "Chain ID"
// @Param duration query string false "Duration: 24h,7d,30d"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/history-sales [get]
func HistorySalesHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chainID, err := strconv.ParseInt(c.Query("chain_id"), 10, 64)
		if err != nil {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chain, ok := chainIDToChain[int(chainID)]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		duration := c.Query("duration")
		if duration != "" {
			validParams := map[string]bool{
				"24h": true,
				"7d":  true,
				"30d": true,
			}
			if ok := validParams[duration]; !ok {
				xzap.WithContext(c).Error("duration parse error: ", zap.String("duration", duration))
				xhttp.Error(c, errcode.ErrInvalidParams)
				return
			}
		} else {
			duration = "7d"
		}

		res, err := service.GetHistorySalesPrice(c.Request.Context(), svcCtx, chain, collectionAddr, duration)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("get history sales price error"))
			return
		}

		xhttp.OkJson(c, struct {
			Result interface{} `json:"result"`
		}{
			Result: res,
		})
	}
}

// @Summary Get item listing info
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param token_id path string true "Token ID"
// @Param chain_id query int true "Chain ID"
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/{token_id}/listing [get]
func ItemListingHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		address, err := middleware.GetAuthUserAddress(c, svcCtx.KvStore)
		if err != nil {
			xhttp.Error(c, err)
			return
		}

		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		tokenID := c.Params.ByName("token_id")
		if tokenID == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chainID, err := strconv.ParseInt(c.Query("chain_id"), 10, 64)
		if err != nil {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chain, ok := chainIDToChain[int(chainID)]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		result, err := service.GetItemListingInfo(c.Request.Context(), svcCtx, chain, collectionAddr, tokenID, address)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("get item listing info error"))
			return
		}

		xhttp.OkJson(c, struct {
			Result interface{} `json:"result"`
		}{Result: result})
	}
}

// @Summary Get item traits
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param token_id path string true "Token ID"
// @Param chain_id query int true "Chain ID"
// @Success 200 {object} types.ItemTraitsResp
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/{token_id}/traits [get]
func ItemTraitsHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		tokenID := c.Params.ByName("token_id")
		if tokenID == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chainID, err := strconv.ParseInt(c.Query("chain_id"), 10, 64)
		if err != nil {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chain, ok := chainIDToChain[int(chainID)]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		itemTraits, err := service.GetItemTraits(c.Request.Context(), svcCtx, chain, collectionAddr, tokenID)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("get item traits error"))
			return
		}

		xhttp.OkJson(c, types.ItemTraitsResp{Result: itemTraits})
	}
}

// @Summary Get item owner
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param token_id path string true "Token ID"
// @Param chain_id query int true "Chain ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/{token_id}/owner [get]
func ItemOwnerHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		tokenID := c.Params.ByName("token_id")
		if tokenID == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chainID, err := strconv.ParseInt(c.Query("chain_id"), 10, 64)
		if err != nil {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chain, ok := chainIDToChain[int(chainID)]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		owner, err := service.GetItemOwner(c.Request.Context(), svcCtx, chainID, chain, collectionAddr, tokenID)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("get item owner error"))
			return
		}

		xhttp.OkJson(c, struct {
			Result interface{} `json:"result"`
		}{
			Result: owner,
		})
	}
}

// @Summary Get item image
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param token_id path string true "Token ID"
// @Param chain_id query int true "Chain ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/{token_id}/image [get]
func GetItemImageHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		tokenID := c.Params.ByName("token_id")
		if tokenID == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chainID, err := strconv.ParseInt(c.Query("chain_id"), 10, 64)
		if err != nil {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chain, ok := chainIDToChain[int(chainID)]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		result, err := service.GetItemImage(c.Request.Context(), svcCtx, chain, collectionAddr, tokenID)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("failed on get item image"))
			return
		}

		xhttp.OkJson(c, struct {
			Result interface{} `json:"result"`
		}{Result: result})
	}
}

// @Summary Refresh item metadata
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param token_id path string true "Token ID"
// @Param chain_id query int true "Chain ID"
// @Success 200 {object} types.CommonResp
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/{token_id}/metadata [post]
func ItemMetadataRefreshHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		chainId, err := strconv.ParseInt(c.Query("chain_id"), 10, 32)
		if err != nil {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chain, ok := chainIDToChain[int(chainId)]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		tokenId := c.Params.ByName("token_id")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		err = service.RefreshItemMetadata(c.Request.Context(), svcCtx, chain, chainId, collectionAddr, tokenId)
		if err != nil {
			xhttp.Error(c, err)
			return
		}

		successStr := "Success to joined the refresh queue and waiting for refresh."
		xhttp.OkJson(c, types.CommonResp{Result: successStr})
	}
}

// @Summary Get collection detail
// @Tags Collection
// @Produce json
// @Param address path string true "Collection address"
// @Param chain_id query int true "Chain ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address} [get]
func CollectionDetailHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		chainID, err := strconv.ParseInt(c.Query("chain_id"), 10, 32)
		if err != nil {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		chain, ok := chainIDToChain[int(chainID)]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}
		res, err := service.GetCollectionDetail(c.Request.Context(), svcCtx, chain, collectionAddr)

		xhttp.OkJson(c, res)
	}
}

// MintNFTHandler 安全的 NFT 铸造接口
// @Summary Mint NFT for collection
// @Tags Collection
// @Accept json
// @Produce json
// @Param address path string true "Collection address"
// @Param request body types.MintRequest true "Mint request"
// @Security BearerAuth
// @Success 200 {object} types.MintResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /collections/{address}/mint [post]
func MintNFTHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 验证用户身份
		address, err := middleware.GetAuthUserAddress(c, svcCtx.KvStore)
		if err != nil {
			xhttp.Error(c, err)
			return
		}

		// 获取合约地址
		collectionAddr := c.Params.ByName("address")
		if collectionAddr == "" {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		// 验证合约地址是否为授权的铸造合约
		if collectionAddr != "0xF7367110305e0419425441e2280eBbAa980A9e42" {
			xhttp.Error(c, errcode.NewCustomErr("Unauthorized contract address"))
			return
		}

		// 解析请求参数
		var mintReq types.MintRequest
		if err := c.ShouldBindJSON(&mintReq); err != nil {
			xhttp.Error(c, errcode.NewCustomErr("Invalid request parameters"))
			return
		}

		// 验证链ID
		chain, ok := chainIDToChain[mintReq.ChainID]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		// 调用铸造服务
		result, err := service.MintNFT(c.Request.Context(), svcCtx, chain, collectionAddr, address[0], &mintReq)
		if err != nil {
			xzap.WithContext(c).Error("mint NFT failed", zap.Error(err))
			xhttp.Error(c, errcode.NewCustomErr("Failed to mint NFT"))
			return
		}

		xhttp.OkJson(c, types.MintResponse{
			Result:        result,
			TransactionID: result.TxHash,
			TokenID:       result.TokenID,
		})
	}
}
