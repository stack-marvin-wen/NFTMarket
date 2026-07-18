package types

import "time"

// NFT 合约管理相关类型 - 对应 ob_collection_* 表结构
type Contract struct {
	ID               int64   `json:"id" gorm:"column:id"`                                 // 主键
	Symbol           string  `json:"symbol" gorm:"column:symbol"`                         // 项目标识
	ChainID          int64   `json:"chain_id" gorm:"column:chain_id"`                     // 链类型
	Auth             int     `json:"auth" gorm:"column:auth"`                             // 认证状态(0:未认证1:认证通过2:认证不通过)
	TokenStandard    int64   `json:"token_standard" gorm:"column:token_standard"`         // 合约实现标准
	Name             string  `json:"name" gorm:"column:name"`                             // 项目名称
	Creator          string  `json:"creator" gorm:"column:creator"`                       // 创建者
	Address          string  `json:"address" gorm:"column:address"`                       // 链上合约地址
	OwnerAmount      int64   `json:"owner_amount" gorm:"column:owner_amount"`             // 拥有item人数
	ItemAmount       int64   `json:"item_amount" gorm:"column:item_amount"`               // NFT发行总量
	Description      *string `json:"description" gorm:"column:description"`               // 项目描述
	Website          *string `json:"website" gorm:"column:website"`                       // 项目官网地址
	Twitter          *string `json:"twitter" gorm:"column:twitter"`                       // 项目twitter地址
	Discord          *string `json:"discord" gorm:"column:discord"`                       // 项目discord地址
	Instagram        *string `json:"instagram" gorm:"column:instagram"`                   // 项目instagram地址
	FloorPrice       *string `json:"floor_price" gorm:"column:floor_price"`               // 地板价
	SalePrice        *string `json:"sale_price" gorm:"column:sale_price"`                 // 最高bid价格
	VolumeTotal      *string `json:"volume_total" gorm:"column:volume_total"`             // 总交易量
	ImageURI         *string `json:"image_uri" gorm:"column:image_uri"`                   // 项目封面图链接
	Banner           *string `json:"banner" gorm:"column:banner"`                         // Banner(视频/图片)链接
	BannerURI        *string `json:"banner_uri" gorm:"column:banner_uri"`                 // banner图片链接
	OpenseaBanScan   int     `json:"opensea_ban_scan" gorm:"column:opensea_ban_scan"`     // opensea扫描状态
	IsSyncing        bool    `json:"is_syncing" gorm:"column:is_syncing"`                 // 是否同步中
	HistorySaleSync  int     `json:"history_sale_sync" gorm:"column:history_sale_sync"`   // 历史销售同步状态
	HistoryOverview  int     `json:"history_overview" gorm:"column:history_overview"`     // 历史概览生成状态
	FloorPriceStatus int     `json:"floor_price_status" gorm:"column:floor_price_status"` // 地板价状态
	CreateTime       *int64  `json:"create_time" gorm:"column:create_time"`               // 创建时间
	UpdateTime       *int64  `json:"update_time" gorm:"column:update_time"`               // 更新时间
}

// 添加合约请求
type AdminAddContractReq struct {
	Address       string  `json:"address" validate:"required"`        // 合约地址
	Name          string  `json:"name" validate:"required"`           // 合约名称
	Symbol        string  `json:"symbol" validate:"required"`         // 项目标识
	ChainID       int64   `json:"chain_id" validate:"required"`       // 链ID
	TokenStandard int64   `json:"token_standard" validate:"required"` // 合约实现标准(721/1155)
	Creator       string  `json:"creator"`                            // 创建者地址
	Description   *string `json:"description"`                        // 描述
	Website       *string `json:"website"`                            // 官网
	Twitter       *string `json:"twitter"`                            // Twitter链接
	Discord       *string `json:"discord"`                            // Discord链接
	Instagram     *string `json:"instagram"`                          // Instagram链接
	ImageURI      *string `json:"image_uri"`                          // 项目图片
	Banner        *string `json:"banner"`                             // Banner(视频/图片)
	BannerURI     *string `json:"banner_uri"`                         // Banner图片
}

// 更新合约请求
type AdminUpdateContractReq struct {
	Name        *string `json:"name"`        // 合约名称
	Symbol      *string `json:"symbol"`      // 项目标识
	Description *string `json:"description"` // 描述
	Website     *string `json:"website"`     // 官网
	Twitter     *string `json:"twitter"`     // Twitter链接
	Discord     *string `json:"discord"`     // Discord链接
	Instagram   *string `json:"instagram"`   // Instagram链接
	ImageURI    *string `json:"image_uri"`   // 项目图片
	Banner      *string `json:"banner"`      // Banner(视频/图片)
	BannerURI   *string `json:"banner_uri"`  // Banner图片
	Auth        *int    `json:"auth"`        // 认证状态
}

// 获取合约列表请求
type AdminGetContractsReq struct {
	Page      int    `form:"page" validate:"min=1"`              // 页码
	PageSize  int    `form:"page_size" validate:"min=1,max=100"` // 页大小
	ChainID   int64  `form:"chain_id"`                           // 链ID筛选
	Auth      *int   `form:"auth"`                               // 认证状态筛选
	IsSyncing *bool  `form:"is_syncing"`                         // 同步状态筛选
	Keyword   string `form:"keyword"`                            // 关键词搜索(名称/地址/符号)
}

// 获取合约列表响应
type AdminGetContractsResp struct {
	Total     int64      `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
	Contracts []Contract `json:"contracts"`
}

// NFT 导入相关类型
type SyncTask struct {
	ID             string     `json:"id"`              // 任务ID
	TaskType       string     `json:"task_type"`       // 任务类型 (contract/token)
	ContractAddr   string     `json:"contract_addr"`   // 合约地址
	TokenID        string     `json:"token_id"`        // Token ID (当任务类型为token时)
	ChainID        int64      `json:"chain_id"`        // 链ID
	Status         string     `json:"status"`          // 状态 (pending/running/completed/failed)
	Progress       int        `json:"progress"`        // 进度 (0-100)
	TotalItems     int        `json:"total_items"`     // 总数量
	ProcessedItems int        `json:"processed_items"` // 已处理数量
	ErrorMsg       string     `json:"error_msg"`       // 错误信息
	CreatedAt      time.Time  `json:"created_at"`      // 创建时间
	StartedAt      *time.Time `json:"started_at"`      // 开始时间
	CompletedAt    *time.Time `json:"completed_at"`    // 完成时间
}

// 同步合约请求
type AdminSyncContractReq struct {
	ContractAddr string `json:"contract_addr" validate:"required"` // 合约地址
	ChainID      int64  `json:"chain_id" validate:"required"`      // 链ID
	StartBlock   int64  `json:"start_block"`                       // 起始区块(可选)
	EndBlock     int64  `json:"end_block"`                         // 结束区块(可选)
}

// 同步单个Token请求
type AdminSyncTokenReq struct {
	ContractAddr string `json:"contract_addr" validate:"required"` // 合约地址
	TokenID      string `json:"token_id" validate:"required"`      // Token ID
	ChainID      int64  `json:"chain_id" validate:"required"`      // 链ID
}

// 同步响应
type AdminSyncResp struct {
	TaskID string `json:"task_id"` // 任务ID
}

// 获取同步状态响应
type AdminGetSyncStatusResp struct {
	Task SyncTask `json:"task"`
}

// 获取同步历史请求
type AdminGetSyncHistoryReq struct {
	Page         int    `form:"page" validate:"min=1"`              // 页码
	PageSize     int    `form:"page_size" validate:"min=1,max=100"` // 页大小
	ContractAddr string `form:"contract_addr"`                      // 合约地址筛选
	Status       string `form:"status"`                             // 状态筛选
	TaskType     string `form:"task_type"`                          // 任务类型筛选
}

// 获取同步历史响应
type AdminGetSyncHistoryResp struct {
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Tasks    []SyncTask `json:"tasks"`
}

// 系统统计
type SystemStats struct {
	TotalContracts   int64 `json:"total_contracts"`   // 总合约数
	EnabledContracts int64 `json:"enabled_contracts"` // 启用的合约数
	TotalNFTs        int64 `json:"total_nfts"`        // 总NFT数量
	TotalUsers       int64 `json:"total_users"`       // 总用户数
	TotalOrders      int64 `json:"total_orders"`      // 总订单数
	RunningTasks     int64 `json:"running_tasks"`     // 运行中的任务数
}

// 获取系统统计响应
type AdminGetSystemStatsResp struct {
	Stats SystemStats `json:"stats"`
}

// 批量刷新元数据请求
type AdminRefreshMetadataReq struct {
	ContractAddr string   `json:"contract_addr" validate:"required"` // 合约地址
	TokenIDs     []string `json:"token_ids"`                         // Token ID列表(可选，为空则刷新所有)
	ChainID      int64    `json:"chain_id" validate:"required"`      // 链ID
}

// 通用响应
type AdminCommonResp struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
