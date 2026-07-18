package v1

import (
	"encoding/json"

	"github.com/ProjectsTask/EasySwapBase/errcode"
	"github.com/ProjectsTask/EasySwapBase/logger/xzap"
	"github.com/ProjectsTask/EasySwapBase/xhttp"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/service/v1"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

// @Summary Get user collections across chains
// @Tags Portfolio
// @Produce json
// @Param filters query string true "Portfolio collection filter JSON string"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /portfolio/collections [get]
func UserMultiChainCollectionsHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		filterParam := c.Query("filters")
		if filterParam == "" {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		var filter types.UserCollectionsParams
		err := json.Unmarshal([]byte(filterParam), &filter)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		var chainNames []string
		var chainIDs []int
		for _, chain := range svcCtx.C.ChainSupported {
			chainIDs = append(chainIDs, chain.ChainID)
			chainNames = append(chainNames, chain.Name)
		}

		res, err := service.GetMultiChainUserCollections(c.Request.Context(), svcCtx, chainIDs, chainNames, filter.UserAddresses)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("query user multi chain collections err."))
			return
		}

		xhttp.OkJson(c, res)
	}
}

// @Summary Get user items across chains
// @Tags Portfolio
// @Produce json
// @Param filters query string true "Portfolio item filter JSON string"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /portfolio/items [get]
func UserMultiChainItemsHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		filterParam := c.Query("filters")
		if filterParam == "" {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		var filter types.PortfolioMultiChainItemFilterParams
		err := json.Unmarshal([]byte(filterParam), &filter)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		// if filter.ChainID is empty, show all chain info
		if len(filter.ChainID) == 0 {
			for _, chain := range svcCtx.C.ChainSupported {
				filter.ChainID = append(filter.ChainID, chain.ChainID)
			}
		}

		var chainNames []string
		for _, chainID := range filter.ChainID {
			chain, ok := chainIDToChain[chainID]
			if !ok {
				xhttp.Error(c, errcode.ErrInvalidParams)
				return
			}
			chainNames = append(chainNames, chain)
		}

		res, err := service.GetMultiChainUserItems(c.Request.Context(), svcCtx, filter.ChainID, chainNames, filter.UserAddresses, filter.CollectionAddresses, filter.Page, filter.PageSize)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("query user multi chain items err."))
			return
		}

		xhttp.OkJson(c, res)
	}
}

// @Summary Get user listings across chains
// @Tags Portfolio
// @Produce json
// @Param filters query string true "Portfolio listing filter JSON string"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /portfolio/listings [get]
func UserMultiChainListingsHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		filterParam := c.Query("filters")
		if filterParam == "" {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		var filter types.PortfolioMultiChainListingFilterParams
		err := json.Unmarshal([]byte(filterParam), &filter)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		// if filter.ChainID is empty, show all chain info
		if len(filter.ChainID) == 0 {
			for _, chain := range svcCtx.C.ChainSupported {
				filter.ChainID = append(filter.ChainID, chain.ChainID)
			}
		}

		var chainNames []string
		for _, chainID := range filter.ChainID {
			chain, ok := chainIDToChain[chainID]
			if !ok {
				xhttp.Error(c, errcode.ErrInvalidParams)
				return
			}
			chainNames = append(chainNames, chain)
		}

		res, err := service.GetMultiChainUserListings(c.Request.Context(), svcCtx, filter.ChainID, chainNames, filter.UserAddresses, filter.CollectionAddresses, filter.Page, filter.PageSize)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("query user multi chain items err."))
			return
		}

		xhttp.OkJson(c, res)
	}
}

// @Summary Get user bids across chains
// @Tags Portfolio
// @Produce json
// @Param filters query string true "Portfolio bid filter JSON string"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /portfolio/bids [get]
func UserMultiChainBidsHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		filterParam := c.Query("filters")
		if filterParam == "" {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		var filter types.PortfolioMultiChainBidFilterParams
		err := json.Unmarshal([]byte(filterParam), &filter)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("Filter param is nil."))
			return
		}

		// if filter.ChainID is empty, show all chain info
		if len(filter.ChainID) == 0 {
			for _, chain := range svcCtx.C.ChainSupported {
				filter.ChainID = append(filter.ChainID, chain.ChainID)
			}
		}

		var chainNames []string
		for _, chainID := range filter.ChainID {
			chain, ok := chainIDToChain[chainID]
			if !ok {
				xhttp.Error(c, errcode.ErrInvalidParams)
				return
			}
			chainNames = append(chainNames, chain)
		}

		res, err := service.GetMultiChainUserBids(c.Request.Context(), svcCtx, filter.ChainID, chainNames, filter.UserAddresses, filter.CollectionAddresses, filter.Page, filter.PageSize)
		if err != nil {
			xhttp.Error(c, errcode.NewCustomErr("query user multi chain items err."))
			return
		}

		xhttp.OkJson(c, res)
	}
}

// @Summary Import one NFT item into portfolio tables
// @Tags Portfolio
// @Accept json
// @Produce json
// @Param request body types.PortfolioImportItemRequest true "Import item request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /portfolio/import-item [post]
func ImportPortfolioItemHandler(svcCtx *svc.ServerCtx) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req types.PortfolioImportItemRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			xhttp.Error(c, errcode.NewCustomErr("invalid request parameters"))
			return
		}

		if err := service.ValidatePortfolioImportRequest(&req); err != nil {
			xhttp.Error(c, errcode.NewCustomErr(err.Error()))
			return
		}

		chain, ok := chainIDToChain[req.ChainID]
		if !ok {
			xhttp.Error(c, errcode.ErrInvalidParams)
			return
		}

		res, err := service.ImportPortfolioItem(c.Request.Context(), svcCtx, chain, &req)
		if err != nil {
			xzap.WithContext(c).Error("failed to import portfolio item",
				zap.Error(err),
				zap.Int("chain_id", req.ChainID),
				zap.String("collection_address", req.CollectionAddress),
				zap.String("token_id", req.TokenID))
			xhttp.Error(c, errcode.NewCustomErr("import item failed: "+err.Error()))
			return
		}
		xhttp.OkJson(c, struct {
			Result interface{} `json:"result"`
		}{Result: res})
	}
}
