package router

import (
	"github.com/gin-gonic/gin"

	"github.com/ProjectsTask/EasySwapBackend/src/api/middleware"
	v1 "github.com/ProjectsTask/EasySwapBackend/src/api/v1"
	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
)

func loadV1(r *gin.Engine, svcCtx *svc.ServerCtx) {
	apiV1 := r.Group("/api/v1")

	user := apiV1.Group("/user")
	{
		user.POST("/login", v1.UserLoginHandler(svcCtx))                       // login
		user.GET("/:address/login-message", v1.GetLoginMessageHandler(svcCtx)) // login msg
		user.GET("/:address/sig-status", v1.GetSigStatusHandler(svcCtx))       // sig status
	}

	// collections
	collections := apiV1.Group("/collections")
	{
		collections.GET("/ranking", middleware.CacheApi(svcCtx.KvStore, 60), v1.TopRankingHandler(svcCtx))
		collections.GET("/:address", v1.CollectionDetailHandler(svcCtx))
		collections.GET("/:address/bids", v1.CollectionBidsHandler(svcCtx))
		collections.GET("/:address/:token_id/bids", v1.CollectionItemBidsHandler(svcCtx))
		collections.GET("/:address/items", v1.CollectionItemsHandler(svcCtx))
		collections.GET("/:address/history-sales", v1.HistorySalesHandler(svcCtx))
		collections.GET("/:address/:token_id/image", middleware.CacheApi(svcCtx.KvStore, 60), v1.GetItemImageHandler(svcCtx))
		collections.GET("/:address/:token_id", v1.ItemDetailHandler(svcCtx))
		collections.GET("/:address/:token_id/traits", v1.ItemTraitsHandler(svcCtx))
		collections.GET("/:address/top-trait", v1.ItemTopTraitPriceHandler(svcCtx))
		collections.GET("/:address/:token_id/owner", v1.ItemOwnerHandler(svcCtx))
		collections.POST("/:address/:token_id/metadata", v1.ItemMetadataRefreshHandler(svcCtx))

		collections.GET("/:address/:token_id/listing", middleware.AuthMiddleWare(svcCtx.KvStore), v1.ItemListingHandler(svcCtx))

		// NFT 铸造接口 - 需要认证
		collections.POST("/:address/mint", middleware.AuthMiddleWare(svcCtx.KvStore), v1.MintNFTHandler(svcCtx))
	}

	activities := apiV1.Group("/activities")
	{
		activities.GET("", v1.ActivityMultiChainHandler(svcCtx))
	}

	portfolio := apiV1.Group("/portfolio")
	{
		portfolio.GET("/collections", v1.UserMultiChainCollectionsHandler(svcCtx))
		portfolio.GET("/items", v1.UserMultiChainItemsHandler(svcCtx))
		portfolio.GET("/listings", v1.UserMultiChainListingsHandler(svcCtx))
		portfolio.GET("/bids", v1.UserMultiChainBidsHandler(svcCtx))
		portfolio.POST("/import-item", v1.ImportPortfolioItemHandler(svcCtx))
	}

	orders := apiV1.Group("/bid-orders")
	{
		orders.GET("", v1.OrderInfosHandler(svcCtx))
	}

	// 腾讯云COS文件上传相关接口
	upload := apiV1.Group("/upload")
	{
		upload.POST("/cos-token", v1.GetCOSTokenHandler(svcCtx))                                                   // 获取COS临时访问凭证（免登录）
		upload.GET("/cos-policy", middleware.AuthMiddleWare(svcCtx.KvStore), v1.GetCOSUploadPolicyHandler(svcCtx)) // 获取COS上传策略（需要认证）
		upload.POST("/cos-callback", middleware.AuthMiddleWare(svcCtx.KvStore), v1.COSCallbackHandler(svcCtx))     // COS上传回调（需要认证）
	}

	// MetaNodeNFT 相关接口
	metanode := apiV1.Group("/metanode")
	{
		metanode.POST("/mint", v1.MetaNodeMintHandler(svcCtx))                 // 单个NFT铸造（免登录）
		metanode.POST("/batch-mint", v1.MetaNodeBatchMintHandler(svcCtx))      // 批量NFT铸造（免登录）
		metanode.GET("/query", v1.MetaNodeQueryHandler(svcCtx))                // 查询NFT信息（免登录）
		metanode.GET("/contract-info", v1.MetaNodeContractInfoHandler(svcCtx)) // 获取合约信息（免登录）
		metanode.GET("/token/:token_id", v1.MetaNodeTokenInfoHandler(svcCtx))  // 获取特定Token信息（免登录）
	}

	// 管理员接口 - 需要管理员权限认证
	admin := apiV1.Group("/admin")
	admin.Use(middleware.AuthMiddleWare(svcCtx.KvStore)) // 需要认证
	// admin.Use(middleware.AdminMiddleware(svcCtx.KvStore)) // 需要管理员权限 (待实现)
	{
		// NFT 合约地址管理 - 支持链参数
		contracts := admin.Group("/contracts")
		{
			contracts.GET("", v1.AdminGetContractsHandler(svcCtx))                                    // 获取合约列表 (支持链ID查询参数)
			contracts.POST("", v1.AdminAddContractHandler(svcCtx))                                    // 添加合约地址
			contracts.GET("/:chain_id/:address", v1.AdminGetContractHandler(svcCtx))                  // 获取单个合约详情
			contracts.PUT("/:chain_id/:address", v1.AdminUpdateContractHandler(svcCtx))               // 更新合约信息
			contracts.DELETE("/:chain_id/:address", v1.AdminDeleteContractHandler(svcCtx))            // 删除合约地址
			contracts.POST("/:chain_id/:address/enable", v1.AdminEnableContractHandler(svcCtx))       // 启用合约
			contracts.POST("/:chain_id/:address/disable", v1.AdminDisableContractHandler(svcCtx))     // 禁用合约
			contracts.POST("/:chain_id/:address/sync", v1.AdminSyncContractFromParamsHandler(svcCtx)) // 通过路径参数同步合约
		}

		// NFT 导入服务
		nftImport := admin.Group("/nft-import")
		{
			nftImport.POST("/sync-contract", v1.AdminSyncContractHandler(svcCtx))        // 同步整个合约的 NFT
			nftImport.POST("/sync-token", v1.AdminSyncTokenHandler(svcCtx))              // 同步指定 Token
			nftImport.GET("/sync-status/:task_id", v1.AdminGetSyncStatusHandler(svcCtx)) // 获取同步状态
			nftImport.GET("/sync-history", v1.AdminGetSyncHistoryHandler(svcCtx))        // 获取同步历史
		}

		// 系统管理
		system := admin.Group("/system")
		{
			system.GET("/stats", v1.AdminGetSystemStatsHandler(svcCtx))              // 获取系统统计
			system.POST("/refresh-metadata", v1.AdminRefreshMetadataHandler(svcCtx)) // 批量刷新元数据
		}
	}
}
