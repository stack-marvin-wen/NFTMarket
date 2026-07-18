package dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

// 根据链名获取 collection 表名
func getCollectionTableName(chainName string) string {
	return fmt.Sprintf("ob_collection_%s", strings.ToLower(chainName))
}

// 根据链ID获取链名 (这里需要根据实际业务调整)
func getChainName(chainID int64) string {
	switch chainID {
	case 1:
		return "ethereum"
	case 11155111:
		return "sepolia"
	case 137:
		return "polygon"
	case 80001:
		return "mumbai"
	default:
		return "sepolia" // 默认使用 sepolia
	}
}

// AdminGetContracts 获取合约列表
func (d *Dao) AdminGetContracts(ctx context.Context, req types.AdminGetContractsReq) ([]types.Contract, int64, error) {
	chainName := getChainName(req.ChainID)
	tableName := getCollectionTableName(chainName)

	var contracts []types.Contract
	var total int64

	// 构建查询条件
	query := d.DB.WithContext(ctx).Table(tableName)

	if req.ChainID > 0 {
		query = query.Where("chain_id = ?", req.ChainID)
	}

	if req.Auth != nil {
		query = query.Where("auth = ?", *req.Auth)
	}

	if req.IsSyncing != nil {
		query = query.Where("is_syncing = ?", *req.IsSyncing)
	}

	// 关键词搜索
	if req.Keyword != "" {
		keyword := "%" + req.Keyword + "%"
		query = query.Where("name LIKE ? OR address LIKE ? OR symbol LIKE ?", keyword, keyword, keyword)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed to count contracts")
	}

	// 分页查询
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("create_time DESC").
		Limit(req.PageSize).
		Offset(offset).
		Find(&contracts).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed to get contracts")
	}

	return contracts, total, nil
}

// AdminAddContract 添加合约
func (d *Dao) AdminAddContract(ctx context.Context, req types.AdminAddContractReq) error {
	chainName := getChainName(req.ChainID)
	tableName := getCollectionTableName(chainName)

	now := time.Now().UnixMilli()

	contract := map[string]interface{}{
		"symbol":             req.Symbol,
		"chain_id":           req.ChainID,
		"auth":               0, // 默认未认证
		"token_standard":     req.TokenStandard,
		"name":               req.Name,
		"creator":            req.Creator,
		"address":            strings.ToLower(req.Address), // 地址转小写
		"owner_amount":       0,
		"item_amount":        0,
		"description":        req.Description,
		"website":            req.Website,
		"twitter":            req.Twitter,
		"discord":            req.Discord,
		"instagram":          req.Instagram,
		"image_uri":          req.ImageURI,
		"banner":             req.Banner,
		"banner_uri":         req.BannerURI,
		"opensea_ban_scan":   0,
		"is_syncing":         false,
		"history_sale_sync":  0,
		"history_overview":   1, // 等待生成
		"floor_price_status": 0,
		"create_time":        now,
		"update_time":        now,
	}

	if err := d.DB.WithContext(ctx).Table(tableName).Create(&contract).Error; err != nil {
		return errors.Wrap(err, "failed to create contract")
	}

	return nil
}

// AdminUpdateContract 更新合约信息
func (d *Dao) AdminUpdateContract(ctx context.Context, chainID int64, address string, req types.AdminUpdateContractReq) error {
	chainName := getChainName(chainID)
	tableName := getCollectionTableName(chainName)

	updates := map[string]interface{}{
		"update_time": time.Now().UnixMilli(),
	}

	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Symbol != nil {
		updates["symbol"] = *req.Symbol
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Website != nil {
		updates["website"] = *req.Website
	}
	if req.Twitter != nil {
		updates["twitter"] = *req.Twitter
	}
	if req.Discord != nil {
		updates["discord"] = *req.Discord
	}
	if req.Instagram != nil {
		updates["instagram"] = *req.Instagram
	}
	if req.ImageURI != nil {
		updates["image_uri"] = *req.ImageURI
	}
	if req.Banner != nil {
		updates["banner"] = *req.Banner
	}
	if req.BannerURI != nil {
		updates["banner_uri"] = *req.BannerURI
	}
	if req.Auth != nil {
		updates["auth"] = *req.Auth
	}

	result := d.DB.WithContext(ctx).Table(tableName).
		Where("address = ?", strings.ToLower(address)).
		Updates(updates)

	if result.Error != nil {
		return errors.Wrap(result.Error, "failed to update contract")
	}

	if result.RowsAffected == 0 {
		return errors.New("contract not found")
	}

	return nil
}

// AdminDeleteContract 删除合约
func (d *Dao) AdminDeleteContract(ctx context.Context, chainID int64, address string) error {
	chainName := getChainName(chainID)
	tableName := getCollectionTableName(chainName)

	result := d.DB.WithContext(ctx).Table(tableName).
		Where("address = ?", strings.ToLower(address)).
		Delete(&types.Contract{})

	if result.Error != nil {
		return errors.Wrap(result.Error, "failed to delete contract")
	}

	if result.RowsAffected == 0 {
		return errors.New("contract not found")
	}

	return nil
}

// AdminGetContract 获取单个合约信息
func (d *Dao) AdminGetContract(ctx context.Context, chainID int64, address string) (*types.Contract, error) {
	chainName := getChainName(chainID)
	tableName := getCollectionTableName(chainName)

	var contract types.Contract
	if err := d.DB.WithContext(ctx).Table(tableName).
		Where("address = ?", strings.ToLower(address)).
		First(&contract).Error; err != nil {
		return nil, errors.Wrap(err, "failed to get contract")
	}

	return &contract, nil
}

// AdminSetContractSyncStatus 设置合约同步状态
func (d *Dao) AdminSetContractSyncStatus(ctx context.Context, chainID int64, address string, isSyncing bool) error {
	chainName := getChainName(chainID)
	tableName := getCollectionTableName(chainName)

	updates := map[string]interface{}{
		"is_syncing":  isSyncing,
		"update_time": time.Now().UnixMilli(),
	}

	result := d.DB.WithContext(ctx).Table(tableName).
		Where("address = ?", strings.ToLower(address)).
		Updates(updates)

	if result.Error != nil {
		return errors.Wrap(result.Error, "failed to update sync status")
	}

	return nil
}

// AdminGetSystemStats 获取系统统计数据
func (d *Dao) AdminGetSystemStats(ctx context.Context) (*types.SystemStats, error) {
	stats := &types.SystemStats{}

	// 获取主要链的统计数据 (以 sepolia 为例)
	chainName := "sepolia"
	tableName := getCollectionTableName(chainName)

	// 总合约数
	if err := d.DB.WithContext(ctx).Table(tableName).Count(&stats.TotalContracts).Error; err != nil {
		return nil, errors.Wrap(err, "failed to count total contracts")
	}

	// 认证通过的合约数
	if err := d.DB.WithContext(ctx).Table(tableName).
		Where("auth = ?", 1).
		Count(&stats.EnabledContracts).Error; err != nil {
		return nil, errors.Wrap(err, "failed to count enabled contracts")
	}

	// 总NFT数量
	var totalNFTs int64
	if err := d.DB.WithContext(ctx).Table(tableName).
		Select("COALESCE(SUM(item_amount), 0)").
		Row().Scan(&totalNFTs); err != nil {
		return nil, errors.Wrap(err, "failed to sum total NFTs")
	}
	stats.TotalNFTs = totalNFTs

	// 正在同步的任务数
	if err := d.DB.WithContext(ctx).Table(tableName).
		Where("is_syncing = ?", true).
		Count(&stats.RunningTasks).Error; err != nil {
		return nil, errors.Wrap(err, "failed to count running tasks")
	}

	// TODO: 添加用户数和订单数的统计逻辑
	// 这需要查询对应的用户表和订单表
	stats.TotalUsers = 0  // 暂时设为0
	stats.TotalOrders = 0 // 暂时设为0

	return stats, nil
}

// AdminCheckContractExists 检查合约是否已存在
func (d *Dao) AdminCheckContractExists(ctx context.Context, chainID int64, address string) (bool, error) {
	chainName := getChainName(chainID)
	tableName := getCollectionTableName(chainName)

	var count int64
	if err := d.DB.WithContext(ctx).Table(tableName).
		Where("address = ?", strings.ToLower(address)).
		Count(&count).Error; err != nil {
		return false, errors.Wrap(err, "failed to check contract existence")
	}

	return count > 0, nil
}
