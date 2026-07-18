package service

import (
	"context"
	"sort"

	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/multi"
	"github.com/pkg/errors"

	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

func GetOrderInfos(ctx context.Context, svcCtx *svc.ServerCtx, chainID int, chain string, userAddr string, collectionAddr string, tokenIds []string) ([]types.ItemBid, error) {
	var items []types.ItemInfo
	for _, tokenID := range tokenIds {
		items = append(items, types.ItemInfo{
			CollectionAddress: collectionAddr,
			TokenID:           tokenID,
		})
	}

	bids, err := svcCtx.Dao.QueryItemsBestBids(ctx, chain, userAddr, items)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query items best bids")
	}

	itemsBestBids := make(map[string]multi.Order)
	for _, bid := range bids {
		order, ok := itemsBestBids[bid.TokenId]
		if !ok {
			itemsBestBids[bid.TokenId] = bid
			continue
		}
		if bid.Price.GreaterThan(order.Price) {
			itemsBestBids[bid.TokenId] = bid
		}
	}

	collectionBids, err := svcCtx.Dao.QueryCollectionTopNBid(ctx, chain, userAddr, collectionAddr, len(tokenIds))
	if err != nil {
		return nil, errors.Wrap(err, "failed on query collection best bids")
	}

	return processBids(tokenIds, itemsBestBids, collectionBids, collectionAddr), nil
}

func processBids(tokenIds []string, itemsBestBids map[string]multi.Order, collectionBids []multi.Order, collectionAddr string) []types.ItemBid {
	var itemsSortedBids []multi.Order
	for _, bid := range itemsBestBids {
		itemsSortedBids = append(itemsSortedBids, bid)
	}

	// Sort the bids slice based on the Price in asc order
	sort.SliceStable(itemsSortedBids, func(i, j int) bool {
		return itemsSortedBids[i].Price.LessThan(itemsSortedBids[j].Price)
	})

	var resultBids []types.ItemBid
	var cBidIndex int
	for _, tokenId := range tokenIds {
		if _, ok := itemsBestBids[tokenId]; !ok {
			if cBidIndex >= len(collectionBids) {
				continue
			} else {
				resultBids = append(resultBids, types.ItemBid{
					MarketplaceId:     collectionBids[cBidIndex].MarketplaceId,
					CollectionAddress: collectionAddr,
					TokenId:           tokenId,
					OrderID:           collectionBids[cBidIndex].OrderID,
					EventTime:         collectionBids[cBidIndex].EventTime,
					ExpireTime:        collectionBids[cBidIndex].ExpireTime,
					Price:             collectionBids[cBidIndex].Price,
					Salt:              collectionBids[cBidIndex].Salt,
					BidSize:           collectionBids[cBidIndex].Size,
					BidUnfilled:       collectionBids[cBidIndex].QuantityRemaining,
					Bidder:            collectionBids[cBidIndex].Maker,
					OrderType:         getBidType(collectionBids[cBidIndex].OrderType),
				})
				cBidIndex++
			}
		}
	}

	for _, itemBid := range itemsSortedBids {
		if cBidIndex >= len(collectionBids) {
			resultBids = append(resultBids, types.ItemBid{
				MarketplaceId:     itemBid.MarketplaceId,
				CollectionAddress: collectionAddr,
				TokenId:           itemBid.TokenId,
				OrderID:           itemBid.OrderID,
				EventTime:         itemBid.EventTime,
				ExpireTime:        itemBid.ExpireTime,
				Price:             itemBid.Price,
				Salt:              itemBid.Salt,
				BidSize:           itemBid.Size,
				BidUnfilled:       itemBid.QuantityRemaining,
				Bidder:            itemBid.Maker,
				OrderType:         getBidType(itemBid.OrderType),
			})
		} else {
			cBid := collectionBids[cBidIndex]
			if cBid.Price.GreaterThan(itemBid.Price) {
				resultBids = append(resultBids, types.ItemBid{
					MarketplaceId:     cBid.MarketplaceId,
					CollectionAddress: collectionAddr,
					TokenId:           itemBid.TokenId,
					OrderID:           cBid.OrderID,
					EventTime:         cBid.EventTime,
					ExpireTime:        cBid.ExpireTime,
					Price:             cBid.Price,
					Salt:              cBid.Salt,
					BidSize:           cBid.Size,
					BidUnfilled:       cBid.QuantityRemaining,
					Bidder:            cBid.Maker,
					OrderType:         getBidType(cBid.OrderType),
				})
				cBidIndex++
			} else {
				resultBids = append(resultBids, types.ItemBid{
					MarketplaceId:     itemBid.MarketplaceId,
					CollectionAddress: collectionAddr,
					TokenId:           itemBid.TokenId,
					OrderID:           itemBid.OrderID,
					EventTime:         itemBid.EventTime,
					ExpireTime:        itemBid.ExpireTime,
					Price:             itemBid.Price,
					Salt:              itemBid.Salt,
					BidSize:           itemBid.Size,
					BidUnfilled:       itemBid.QuantityRemaining,
					Bidder:            itemBid.Maker,
					OrderType:         getBidType(itemBid.OrderType),
				})
			}
		}
	}

	return resultBids
}
