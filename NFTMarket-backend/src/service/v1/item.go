package service

import (
	"context"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"

	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

func GetItemListingInfo(ctx context.Context, svcCtx *svc.ServerCtx, chain, collectionAddr, tokenId string, user []string) ([]types.ListingInfo, error) {
	listings, err := svcCtx.Dao.QueryItemListingAcrossPlatforms(ctx, chain, collectionAddr, tokenId, user)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query from db")
	}

	var result []types.ListingInfo
	for _, listing := range listings {
		if listing.Price.GreaterThan(decimal.NewFromInt(0)) {
			result = append(result, listing)
		}
	}

	return result, nil
}

func GetItemBidsInfo(ctx context.Context, svcCtx *svc.ServerCtx, chain string, collectionAddr, tokenID string, page, pageSize int) (*types.CollectionBidsResp, error) {
	bids, count, err := svcCtx.Dao.QueryItemBids(ctx, chain, collectionAddr, tokenID, page, pageSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed on get item info")
	}

	for i := 0; i < len(bids); i++ {
		bids[i].OrderType = getBidType(bids[i].OrderType)
	}
	return &types.CollectionBidsResp{
		Result: bids,
		Count:  count,
	}, nil
}
