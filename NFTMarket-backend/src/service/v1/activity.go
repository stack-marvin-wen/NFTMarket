package service

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

func GetAllChainActivities(ctx context.Context, svcCtx *svc.ServerCtx, collectionAddrs []string, tokenID string, userAddrs []string, eventTypes []string, page, pageSize int) (*types.ActivityResp, error) {
	activities, total, err := svcCtx.Dao.QueryAllChainActivities(ctx, svcCtx.C.ChainSupported, collectionAddrs, tokenID, userAddrs, eventTypes, page, pageSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query multi-chain activity")
	}

	if total == 0 || len(activities) == 0 {
		return &types.ActivityResp{
			Result: nil,
			Count:  0,
		}, nil
	}

	//external info query
	results, err := svcCtx.Dao.QueryAllChainActivityExternalInfo(ctx, svcCtx.C.ChainSupported, activities)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query activity external info")
	}
	for i := 0; i < len(results); i++ {
		if results[i].ImageURI != "" {
			results[i].ImageURI = results[i].ImageURI // svcCtx.ImageMgr.GetSmallSizeImageUrl(results[i].ImageURI)
		}
	}

	return &types.ActivityResp{
		Result: results,
		Count:  total,
	}, nil
}

func GetMultiChainActivities(ctx context.Context, svcCtx *svc.ServerCtx, chainID []int, chainName []string, collectionAddrs []string, tokenID string, userAddrs []string, eventTypes []string, page, pageSize int) (*types.ActivityResp, error) {
	activities, total, err := svcCtx.Dao.QueryMultiChainActivities(ctx, chainName, collectionAddrs, tokenID, userAddrs, eventTypes, page, pageSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query multi-chain activity")
	}

	if total == 0 || len(activities) == 0 {
		return &types.ActivityResp{
			Result: nil,
			Count:  0,
		}, nil
	}

	//external info query
	results, err := svcCtx.Dao.QueryMultiChainActivityExternalInfo(ctx, chainID, chainName, activities)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query activity external info")
	}
	for i := 0; i < len(results); i++ {
		results[i].ImageURI = results[i].ImageURI // svcCtx.ImageMgr.GetSmallSizeImageUrl(results[i].ImageURI)
	}

	return &types.ActivityResp{
		Result: results,
		Count:  total,
	}, nil
}
