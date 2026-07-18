package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

// =================== NFT 合约管理 ===================

// AdminGetContracts 获取合约列表
func AdminGetContracts(ctx context.Context, svcCtx *svc.ServerCtx, req types.AdminGetContractsReq) (*types.AdminGetContractsResp, error) {
	// 如果没有指定链ID，默认使用 sepolia
	if req.ChainID == 0 {
		req.ChainID = 11155111
	}

	contracts, total, err := svcCtx.Dao.AdminGetContracts(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contracts from dao")
	}

	resp := &types.AdminGetContractsResp{
		Total:     total,
		Page:      req.Page,
		PageSize:  req.PageSize,
		Contracts: contracts,
	}

	return resp, nil
}

// AdminGetContract 获取单个合约详情
func AdminGetContract(ctx context.Context, svcCtx *svc.ServerCtx, chainID int64, address string) (*types.Contract, error) {
	// 验证地址格式
	if !isValidAddress(address) {
		return nil, errors.New("invalid contract address format")
	}

	contract, err := svcCtx.Dao.AdminGetContract(ctx, chainID, address)
	if err != nil {
		if strings.Contains(err.Error(), "record not found") {
			return nil, errors.New("contract not found")
		}
		return nil, errors.Wrap(err, "failed to get contract")
	}

	return contract, nil
}

// AdminAddContract 添加合约地址
func AdminAddContract(ctx context.Context, svcCtx *svc.ServerCtx, req types.AdminAddContractReq) (*types.AdminCommonResp, error) {
	// 验证合约地址格式
	if !isValidAddress(req.Address) {
		return nil, errors.New("invalid contract address format")
	}

	// 检查合约是否已存在
	exists, err := svcCtx.Dao.AdminCheckContractExists(ctx, req.ChainID, req.Address)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check contract existence")
	}
	if exists {
		return nil, errors.New("contract already exists")
	}

	// 添加合约
	if err := svcCtx.Dao.AdminAddContract(ctx, req); err != nil {
		return nil, errors.Wrap(err, "failed to add contract")
	}

	return &types.AdminCommonResp{
		Success: true,
		Message: "合约添加成功",
	}, nil
}

// AdminUpdateContract 更新合约信息 (使用默认链ID)
func AdminUpdateContract(ctx context.Context, svcCtx *svc.ServerCtx, address string, req types.AdminUpdateContractReq) (*types.AdminCommonResp, error) {
	// 默认使用 sepolia 链
	chainID := int64(11155111)
	return AdminUpdateContractWithChainID(ctx, svcCtx, chainID, address, req)
}

// AdminUpdateContractWithChainID 更新合约信息 (指定链ID)
func AdminUpdateContractWithChainID(ctx context.Context, svcCtx *svc.ServerCtx, chainID int64, address string, req types.AdminUpdateContractReq) (*types.AdminCommonResp, error) {
	// 验证地址格式
	if !isValidAddress(address) {
		return nil, errors.New("invalid contract address format")
	}

	if err := svcCtx.Dao.AdminUpdateContract(ctx, chainID, address, req); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errors.New("contract not found")
		}
		return nil, errors.Wrap(err, "failed to update contract")
	}

	return &types.AdminCommonResp{
		Success: true,
		Message: "合约更新成功",
	}, nil
}

// AdminDeleteContract 删除合约地址 (使用默认链ID)
func AdminDeleteContract(ctx context.Context, svcCtx *svc.ServerCtx, address string) (*types.AdminCommonResp, error) {
	// 默认使用 sepolia 链
	chainID := int64(11155111)
	return AdminDeleteContractWithChainID(ctx, svcCtx, chainID, address)
}

// AdminDeleteContractWithChainID 删除合约地址 (指定链ID)
func AdminDeleteContractWithChainID(ctx context.Context, svcCtx *svc.ServerCtx, chainID int64, address string) (*types.AdminCommonResp, error) {
	// 验证地址格式
	if !isValidAddress(address) {
		return nil, errors.New("invalid contract address format")
	}

	if err := svcCtx.Dao.AdminDeleteContract(ctx, chainID, address); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errors.New("contract not found")
		}
		return nil, errors.Wrap(err, "failed to delete contract")
	}

	return &types.AdminCommonResp{
		Success: true,
		Message: "合约删除成功",
	}, nil
}

// AdminToggleContract 启用/禁用合约 (使用默认链ID)
func AdminToggleContract(ctx context.Context, svcCtx *svc.ServerCtx, address string, enabled bool) (*types.AdminCommonResp, error) {
	// 默认使用 sepolia 链
	chainID := int64(11155111)
	return AdminToggleContractWithChainID(ctx, svcCtx, chainID, address, enabled)
}

// AdminToggleContractWithChainID 启用/禁用合约 (指定链ID)
func AdminToggleContractWithChainID(ctx context.Context, svcCtx *svc.ServerCtx, chainID int64, address string, enabled bool) (*types.AdminCommonResp, error) {
	// 验证地址格式
	if !isValidAddress(address) {
		return nil, errors.New("invalid contract address format")
	}

	auth := 2 // 认证不通过
	status := "禁用"
	if enabled {
		auth = 1 // 认证通过
		status = "启用"
	}

	updateReq := types.AdminUpdateContractReq{
		Auth: &auth,
	}

	if err := svcCtx.Dao.AdminUpdateContract(ctx, chainID, address, updateReq); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errors.New("contract not found")
		}
		return nil, errors.Wrap(err, "failed to toggle contract status")
	}

	return &types.AdminCommonResp{
		Success: true,
		Message: fmt.Sprintf("合约%s成功", status),
	}, nil
}

// =================== NFT 导入服务 ===================

// AdminSyncContract 同步整个合约的 NFT
func AdminSyncContract(ctx context.Context, svcCtx *svc.ServerCtx, req types.AdminSyncContractReq) (*types.AdminSyncResp, error) {
	// 验证合约地址
	if !isValidAddress(req.ContractAddr) {
		return nil, errors.New("invalid contract address format")
	}

	// 检查合约是否存在
	exists, err := svcCtx.Dao.AdminCheckContractExists(ctx, req.ChainID, req.ContractAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check contract existence")
	}
	if !exists {
		return nil, errors.New("contract not found")
	}

	taskID := uuid.NewString()

	// 设置合约为同步中状态
	if err := svcCtx.Dao.AdminSetContractSyncStatus(ctx, req.ChainID, req.ContractAddr, true); err != nil {
		return nil, errors.Wrap(err, "failed to set sync status")
	}

	// TODO: 创建同步任务记录到专门的任务表
	// 这里暂时只是模拟

	// 异步执行同步任务
	go func() {
		defer func() {
			// 同步完成后，重置同步状态
			svcCtx.Dao.AdminSetContractSyncStatus(context.Background(), req.ChainID, req.ContractAddr, false)
		}()

		// TODO: 实现实际的同步逻辑
		// 1. 从区块链获取合约的 Transfer 事件
		// 2. 解析事件获取 NFT 信息
		// 3. 获取元数据
		// 4. 保存到数据库
		// 5. 更新任务状态和进度

		// 模拟同步过程
		time.Sleep(5 * time.Second)
	}()

	return &types.AdminSyncResp{
		TaskID: taskID,
	}, nil
}

// AdminSyncToken 同步指定 Token
func AdminSyncToken(ctx context.Context, svcCtx *svc.ServerCtx, req types.AdminSyncTokenReq) (*types.AdminSyncResp, error) {
	// 验证合约地址
	if !isValidAddress(req.ContractAddr) {
		return nil, errors.New("invalid contract address format")
	}

	// 检查合约是否存在
	exists, err := svcCtx.Dao.AdminCheckContractExists(ctx, req.ChainID, req.ContractAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check contract existence")
	}
	if !exists {
		return nil, errors.New("contract not found")
	}

	taskID := uuid.NewString()

	// TODO: 创建同步任务记录到专门的任务表

	// 异步执行同步任务
	go func() {
		// TODO: 实现实际的同步逻辑
		// 1. 获取指定 Token 的信息
		// 2. 获取元数据
		// 3. 保存到数据库
		// 4. 更新任务状态

		// 模拟同步过程
		time.Sleep(2 * time.Second)
	}()

	return &types.AdminSyncResp{
		TaskID: taskID,
	}, nil
}

// AdminGetSyncStatus 获取同步状态
func AdminGetSyncStatus(ctx context.Context, svcCtx *svc.ServerCtx, taskID string) (*types.AdminGetSyncStatusResp, error) {
	// TODO: 从数据库查询任务状态
	// 这里先返回模拟数据

	// 模拟数据
	task := types.SyncTask{
		ID:             taskID,
		TaskType:       "contract",
		ContractAddr:   "0x1234567890123456789012345678901234567890",
		ChainID:        11155111,
		Status:         "running",
		Progress:       65,
		TotalItems:     1000,
		ProcessedItems: 650,
		CreatedAt:      time.Now().Add(-30 * time.Minute),
		StartedAt:      &time.Time{},
	}
	*task.StartedAt = time.Now().Add(-25 * time.Minute)

	return &types.AdminGetSyncStatusResp{
		Task: task,
	}, nil
}

// AdminGetSyncHistory 获取同步历史
func AdminGetSyncHistory(ctx context.Context, svcCtx *svc.ServerCtx, req types.AdminGetSyncHistoryReq) (*types.AdminGetSyncHistoryResp, error) {
	// TODO: 从数据库分页查询同步任务历史
	// 这里先返回模拟数据

	// 模拟数据
	tasks := []types.SyncTask{
		{
			ID:             "task-001",
			TaskType:       "contract",
			ContractAddr:   "0x1234567890123456789012345678901234567890",
			ChainID:        11155111,
			Status:         "completed",
			Progress:       100,
			TotalItems:     1000,
			ProcessedItems: 1000,
			CreatedAt:      time.Now().Add(-2 * time.Hour),
			StartedAt:      &time.Time{},
			CompletedAt:    &time.Time{},
		},
	}
	*tasks[0].StartedAt = time.Now().Add(-2 * time.Hour)
	*tasks[0].CompletedAt = time.Now().Add(-1 * time.Hour)

	resp := &types.AdminGetSyncHistoryResp{
		Total:    1,
		Page:     req.Page,
		PageSize: req.PageSize,
		Tasks:    tasks,
	}

	return resp, nil
}

// =================== 系统管理 ===================

// AdminGetSystemStats 获取系统统计
func AdminGetSystemStats(ctx context.Context, svcCtx *svc.ServerCtx) (*types.AdminGetSystemStatsResp, error) {
	stats, err := svcCtx.Dao.AdminGetSystemStats(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get system stats")
	}

	return &types.AdminGetSystemStatsResp{
		Stats: *stats,
	}, nil
}

// AdminRefreshMetadata 批量刷新元数据
func AdminRefreshMetadata(ctx context.Context, svcCtx *svc.ServerCtx, req types.AdminRefreshMetadataReq) (*types.AdminCommonResp, error) {
	// 验证合约地址
	if !isValidAddress(req.ContractAddr) {
		return nil, errors.New("invalid contract address format")
	}

	// 检查合约是否存在
	exists, err := svcCtx.Dao.AdminCheckContractExists(ctx, req.ChainID, req.ContractAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check contract existence")
	}
	if !exists {
		return nil, errors.New("contract not found")
	}

	// 异步执行刷新任务
	go func() {
		// TODO: 实现实际的元数据刷新逻辑
		// 1. 重新获取 NFT 元数据
		// 2. 更新数据库中的元数据信息
		// 3. 更新缓存

		// 模拟刷新过程
		time.Sleep(3 * time.Second)
	}()

	message := fmt.Sprintf("已开始刷新合约 %s 的元数据", req.ContractAddr)
	if len(req.TokenIDs) > 0 {
		message = fmt.Sprintf("已开始刷新合约 %s 的 %d 个 Token 的元数据", req.ContractAddr, len(req.TokenIDs))
	}

	return &types.AdminCommonResp{
		Success: true,
		Message: message,
	}, nil
}

// =================== 辅助函数 ===================

// isValidAddress 验证以太坊地址格式
func isValidAddress(address string) bool {
	if len(address) != 42 {
		return false
	}
	if !strings.HasPrefix(strings.ToLower(address), "0x") {
		return false
	}
	// 这里可以添加更严格的地址格式验证
	return true
}
