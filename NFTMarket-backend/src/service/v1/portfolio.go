package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/multi"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"

	"github.com/ProjectsTask/EasySwapBackend/src/dao"
	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

const BidTypeOffset = 3

func getBidType(origin int64) int64 {
	if origin >= BidTypeOffset {
		return origin - BidTypeOffset
	} else {
		return origin
	}
}

func GetMultiChainUserCollections(ctx context.Context, svcCtx *svc.ServerCtx, chainIDs []int, chainNames []string, userAddrs []string) (*types.UserCollectionsResp, error) {
	collections, err := svcCtx.Dao.QueryMultiChainUserCollectionInfos(ctx, chainIDs, chainNames, userAddrs)
	if err != nil {
		return nil, errors.Wrap(err, "failed on get collection info")
	}

	chainIDToChainName := make(map[int]string)
	for _, chain := range svcCtx.C.ChainSupported {
		chainIDToChainName[chain.ChainID] = chain.Name
	}

	// chainID --> collectionAddrs
	chainIDToCollectionAddrs := make(map[int][]string)
	for _, collection := range collections {
		if _, ok := chainIDToCollectionAddrs[collection.ChainID]; !ok {
			chainIDToCollectionAddrs[collection.ChainID] = []string{collection.Address}
		} else {
			chainIDToCollectionAddrs[collection.ChainID] = append(chainIDToCollectionAddrs[collection.ChainID], collection.Address)
		}
	}

	//for each collection, create a goroutine query corresponding info
	var listed []types.CollectionInfo
	var wg sync.WaitGroup
	var mu sync.Mutex
	for chainID, collectionAddrs := range chainIDToCollectionAddrs {
		chainName := chainIDToChainName[chainID]
		wg.Add(1)
		go func(chainName string, collectionAddrs []string) {
			defer wg.Done()

			list, err := svcCtx.Dao.QueryListedAmountEachCollection(ctx, chainName, collectionAddrs, userAddrs)
			if err != nil {
				return
			}
			mu.Lock()
			listed = append(listed, list...)
			mu.Unlock()
		}(chainName, collectionAddrs)
	}
	wg.Wait()

	collectionsListed := make(map[string]int)
	for _, l := range listed {
		collectionsListed[strings.ToLower(l.Address)] = l.ListAmount
	}

	var results types.UserCollectionsData
	chainInfos := make(map[int]types.ChainInfo)
	for _, collection := range collections {
		listCount := collectionsListed[strings.ToLower(collection.Address)]
		results.CollectionInfos = append(results.CollectionInfos, types.CollectionInfo{
			ChainID:    collection.ChainID,
			Name:       collection.Name,
			Address:    collection.Address,
			Symbol:     collection.Symbol,
			ImageURI:   collection.ImageURI,
			ListAmount: listCount,
			ItemAmount: collection.ItemCount,
			FloorPrice: collection.FloorPrice,
		})

		chainInfo, ok := chainInfos[collection.ChainID]
		if ok {
			chainInfo.ItemOwned += collection.ItemCount
			chainInfo.ItemValue = chainInfo.ItemValue.Add(decimal.New(collection.ItemCount, 0).Mul(collection.FloorPrice))
			chainInfos[collection.ChainID] = chainInfo
		} else {
			chainInfos[collection.ChainID] = types.ChainInfo{
				ChainID:   collection.ChainID,
				ItemOwned: collection.ItemCount,
				ItemValue: decimal.New(collection.ItemCount, 0).Mul(collection.FloorPrice),
			}
		}
	}

	for _, chainInfo := range chainInfos {
		results.ChainInfos = append(results.ChainInfos, chainInfo)
	}

	return &types.UserCollectionsResp{
		Result: results,
	}, nil
}

func GetMultiChainUserItems(ctx context.Context, svcCtx *svc.ServerCtx, chainID []int, chain []string, userAddrs []string, contractAddrs []string, page, pageSize int) (*types.UserItemsResp, error) {
	items, count, err := svcCtx.Dao.QueryMultiChainUserItemInfos(ctx, chain, userAddrs, contractAddrs, page, pageSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed on get user items info")
	}

	if count == 0 {
		return &types.UserItemsResp{
			Result: items,
			Count:  count,
		}, nil
	}

	chainIDToChainName := make(map[int]string)
	for i, _ := range chainID {
		chainIDToChainName[chainID[i]] = chain[i]
	}
	fallbackChainName := ""
	if len(chain) > 0 {
		fallbackChainName = chain[0]
	}
	resolveChainName := func(id int) string {
		chainName := strings.TrimSpace(chainIDToChainName[id])
		if chainName == "" {
			chainName = strings.TrimSpace(fallbackChainName)
		}
		return chainName
	}

	var collectionAddrs [][]string         //{collectionAddr,chainName} for each element
	var itemInfos []dao.MultiChainItemInfo //add chain name info
	var chainCollections = make(map[string][]string)
	var multichainItems = make(map[string][]types.ItemInfo)

	for _, item := range items {
		itemChainName := resolveChainName(item.ChainID)
		if itemChainName == "" {
			continue
		}
		collectionAddrs = append(collectionAddrs, []string{strings.ToLower(item.CollectionAddress), itemChainName})
		itemInfos = append(itemInfos, dao.MultiChainItemInfo{
			ItemInfo: types.ItemInfo{
				CollectionAddress: item.CollectionAddress,
				TokenID:           item.TokenID,
			},
			ChainName: itemChainName,
		})
		chainCollections[strings.ToLower(itemChainName)] = append(chainCollections[strings.ToLower(itemChainName)], item.CollectionAddress)
		multichainItems[itemChainName] = append(multichainItems[itemChainName], types.ItemInfo{
			CollectionAddress: item.CollectionAddress,
			TokenID:           item.TokenID,
		})
	}

	var userAddr string
	if len(userAddrs) == 0 {
		userAddr = ""
	} else {
		userAddr = userAddrs[0]
	}

	collectionBestBids := make(map[types.MultichainCollection]multi.Order)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var queryErr error
	for chain, collections := range chainCollections {
		wg.Add(1)
		go func(chainName string, collectionArray []string) {
			defer wg.Done()
			bestBids, err := svcCtx.Dao.QueryCollectionsBestBid(ctx, chainName, userAddr, collectionArray)
			if err != nil {
				queryErr = errors.Wrap(err, "failed on query collections best bids")
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, bestBid := range bestBids {
				collectionBestBids[types.MultichainCollection{
					CollectionAddress: strings.ToLower(bestBid.CollectionAddress),
					Chain:             chainName,
				}] = *bestBid
			}
		}(chain, collections)
	}
	wg.Wait()
	if queryErr != nil {
		return nil, errors.Wrap(queryErr, "failed on query collection bids")
	}

	itemsBestBids := make(map[dao.MultiChainItemInfo]multi.Order)
	for chain, items := range multichainItems {
		wg.Add(1)
		go func(chainName string, itemInfos []types.ItemInfo) {
			defer wg.Done()
			bids, err := svcCtx.Dao.QueryItemsBestBids(ctx, chainName, userAddr, itemInfos)
			if err != nil {
				queryErr = errors.Wrap(err, "failed on query items best bids")
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, bid := range bids {
				order, ok := itemsBestBids[dao.MultiChainItemInfo{ItemInfo: types.ItemInfo{CollectionAddress: strings.ToLower(bid.CollectionAddress), TokenID: bid.TokenId}, ChainName: chainName}]
				if !ok {
					itemsBestBids[dao.MultiChainItemInfo{ItemInfo: types.ItemInfo{CollectionAddress: strings.ToLower(bid.CollectionAddress), TokenID: bid.TokenId}, ChainName: chainName}] = bid
					continue
				}
				if bid.Price.GreaterThan(order.Price) {
					itemsBestBids[dao.MultiChainItemInfo{ItemInfo: types.ItemInfo{CollectionAddress: strings.ToLower(bid.CollectionAddress), TokenID: bid.TokenId}, ChainName: chainName}] = bid
				}
			}
		}(chain, items)
	}
	wg.Wait()
	if queryErr != nil {
		return nil, errors.Wrap(queryErr, "failed on query items best bids")
	}

	collections, err := svcCtx.Dao.QueryMultiChainCollectionsInfo(ctx, collectionAddrs)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query collections info")
	}

	collectionInfos := make(map[string]multi.Collection)
	for _, collection := range collections {
		collectionInfos[strings.ToLower(collection.Address)] = collection
	}

	listings, err := svcCtx.Dao.QueryMultiChainUserItemsListInfo(ctx, userAddrs, itemInfos)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query item list info")
	}

	listingInfos := make(map[string]*dao.CollectionItem)
	for _, listing := range listings {
		listingInfos[strings.ToLower(listing.CollectionAddress+listing.TokenId)] = listing
	}

	var itemPrice []dao.MultiChainItemPriceInfo
	for _, item := range listingInfos {
		if item.Listing {
			chainName, ok := chainIDToChainName[item.ChainId]
			if !ok || strings.TrimSpace(chainName) == "" {
				continue
			}
			itemPrice = append(itemPrice, dao.MultiChainItemPriceInfo{
				ItemPriceInfo: types.ItemPriceInfo{
					CollectionAddress: item.CollectionAddress,
					TokenID:           item.TokenId,
					Maker:             item.Owner,
					Price:             item.ListPrice,
					OrderStatus:       multi.OrderStatusActive,
				},
				ChainName: chainName,
			})
		}
	}

	orderIds := make(map[string]multi.Order)
	if len(itemPrice) > 0 {
		orders, err := svcCtx.Dao.QueryMultiChainListingInfo(ctx, itemPrice)
		if err != nil {
			return nil, errors.Wrap(err, "failed on query item order id")
		}

		for _, order := range orders {
			orderIds[strings.ToLower(order.CollectionAddress+order.TokenId)] = order
		}

	}

	itemImages, err := svcCtx.Dao.QueryMultiChainCollectionsItemsImage(ctx, itemInfos)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query item image info")
	}

	itemExternals := make(map[string]multi.ItemExternal)
	for _, item := range itemImages {
		itemExternals[strings.ToLower(item.CollectionAddress+item.TokenId)] = item
	}

	for i := 0; i < len(items); i++ {
		bidOrder, ok := itemsBestBids[dao.MultiChainItemInfo{ItemInfo: types.ItemInfo{CollectionAddress: strings.ToLower(items[i].CollectionAddress), TokenID: items[i].TokenID}, ChainName: chainIDToChainName[items[i].ChainID]}]
		if ok {
			if bidOrder.Price.GreaterThan(collectionBestBids[types.MultichainCollection{
				CollectionAddress: strings.ToLower(items[i].CollectionAddress),
				Chain:             chainIDToChainName[items[i].ChainID],
			}].Price) {
				items[i].BidOrderID = bidOrder.OrderID
				items[i].BidExpireTime = bidOrder.ExpireTime
				items[i].BidPrice = bidOrder.Price
				items[i].BidTime = bidOrder.EventTime
				items[i].BidSalt = bidOrder.Salt
				items[i].BidMaker = bidOrder.Maker
				items[i].BidType = getBidType(bidOrder.OrderType)
				items[i].BidSize = bidOrder.Size
				items[i].BidUnfilled = bidOrder.QuantityRemaining
			}
		} else {
			if cBid, ok := collectionBestBids[types.MultichainCollection{
				CollectionAddress: strings.ToLower(items[i].CollectionAddress),
				Chain:             chainIDToChainName[items[i].ChainID],
			}]; ok {
				items[i].BidOrderID = cBid.OrderID
				items[i].BidExpireTime = cBid.ExpireTime
				items[i].BidPrice = cBid.Price
				items[i].BidTime = cBid.EventTime
				items[i].BidSalt = cBid.Salt
				items[i].BidMaker = cBid.Maker
				items[i].BidType = getBidType(cBid.OrderType)
				items[i].BidSize = cBid.Size
				items[i].BidUnfilled = cBid.QuantityRemaining
			}
		}

		collection, ok := collectionInfos[strings.ToLower(items[i].CollectionAddress)]
		if ok {
			items[i].CollectionName = collection.Name
			items[i].FloorPrice = collection.FloorPrice
			items[i].CollectionImageURI = collection.ImageUri
			if items[i].Name == "" {
				items[i].Name = fmt.Sprintf("%s #%s", collection.Name, items[i].TokenID)
			}
		}
		listing, ok := listingInfos[strings.ToLower(items[i].CollectionAddress+items[i].TokenID)]
		if ok {
			items[i].ListPrice = listing.ListPrice
			items[i].Listing = listing.Listing
			items[i].MarketplaceID = listing.MarketID
		}

		order, ok := orderIds[strings.ToLower(items[i].CollectionAddress+items[i].TokenID)]
		if ok {
			items[i].ListOrderID = order.OrderID
			items[i].ListTime = order.EventTime
			items[i].ListExpireTime = order.ExpireTime
			items[i].ListSalt = order.Salt
			items[i].ListMaker = order.Maker
		}

		image, ok := itemExternals[strings.ToLower(items[i].CollectionAddress+items[i].TokenID)]
		if ok {
			if image.IsUploadedOss {
				items[i].ImageURI = image.OssUri // svcCtx.ImageMgr.GetSmallSizeImageUrl(image.OssUri)
			} else {
				items[i].ImageURI = image.ImageUri // svcCtx.ImageMgr.GetSmallSizeImageUrl(image.ImageUri)
			}
		}
	}

	return &types.UserItemsResp{
		Result: items,
		Count:  count,
	}, nil
}

func GetMultiChainUserListings(ctx context.Context, svcCtx *svc.ServerCtx, chainID []int, chain []string, userAddrs []string, contractAddrs []string, page, pageSize int) (*types.UserListingsResp, error) {
	var result []types.Listing
	items, count, err := svcCtx.Dao.QueryMultiChainUserListingItemInfos(ctx, chain, userAddrs, contractAddrs, page, pageSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed on get user items info")
	}

	if count == 0 {
		return &types.UserListingsResp{
			Count: count,
		}, nil
	}

	chainIDToChainName := make(map[int]string)
	for i, _ := range chainID {
		chainIDToChainName[chainID[i]] = chain[i]
	}
	fallbackChainName := ""
	if len(chain) > 0 {
		fallbackChainName = chain[0]
	}
	resolveChainName := func(id int) string {
		chainName := strings.TrimSpace(chainIDToChainName[id])
		if chainName == "" {
			chainName = strings.TrimSpace(fallbackChainName)
		}
		return chainName
	}

	var userAddr string
	if len(userAddrs) == 0 {
		userAddr = ""
	} else {
		userAddr = userAddrs[0]
	}

	var collectionAddrs [][]string         //{collectionAddr,chainName} for each element
	var itemInfos []dao.MultiChainItemInfo //add chain name info
	var chainCollections = make(map[string][]string)
	var multichainItems = make(map[string][]types.ItemInfo)
	for _, item := range items {
		itemChainName := resolveChainName(item.ChainID)
		if itemChainName == "" {
			continue
		}
		collectionAddrs = append(collectionAddrs, []string{strings.ToLower(item.CollectionAddress), itemChainName})
		itemInfos = append(itemInfos, dao.MultiChainItemInfo{
			ItemInfo: types.ItemInfo{
				CollectionAddress: item.CollectionAddress,
				TokenID:           item.TokenID,
			},
			ChainName: itemChainName,
		})

		chainCollections[strings.ToLower(itemChainName)] = append(chainCollections[strings.ToLower(itemChainName)], item.CollectionAddress)
		multichainItems[itemChainName] = append(multichainItems[itemChainName], types.ItemInfo{
			CollectionAddress: item.CollectionAddress,
			TokenID:           item.TokenID,
		})
	}

	itemLastCost := make(map[dao.MultiChainItemInfo]decimal.Decimal)
	collectionBestBids := make(map[types.MultichainCollection]multi.Order)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var queryErr error
	for chain, collections := range chainCollections {
		wg.Add(1)
		go func(chainName string, collectionArray []string) {
			defer wg.Done()
			bestBids, err := svcCtx.Dao.QueryCollectionsBestBid(ctx, chainName, userAddr, collectionArray)
			if err != nil {
				queryErr = errors.Wrap(err, "failed on query collections best bids")
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, bestBid := range bestBids {
				collectionBestBids[types.MultichainCollection{
					CollectionAddress: strings.ToLower(bestBid.CollectionAddress),
					Chain:             chainName,
				}] = *bestBid
			}
		}(chain, collections)
	}
	wg.Wait()
	if queryErr != nil {
		return nil, errors.Wrap(queryErr, "failed on query collection bids")
	}

	itemsBestBids := make(map[dao.MultiChainItemInfo]multi.Order)
	for chain, items := range multichainItems {
		wg.Add(1)
		go func(chainName string, itemInfos []types.ItemInfo) {
			defer wg.Done()
			bids, err := svcCtx.Dao.QueryItemsBestBids(ctx, chainName, userAddr, itemInfos)
			if err != nil {
				queryErr = errors.Wrap(err, "failed on query items best bids")
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, bid := range bids {
				order, ok := itemsBestBids[dao.MultiChainItemInfo{ItemInfo: types.ItemInfo{CollectionAddress: strings.ToLower(bid.CollectionAddress), TokenID: bid.TokenId}, ChainName: chainName}]
				if !ok {
					itemsBestBids[dao.MultiChainItemInfo{ItemInfo: types.ItemInfo{CollectionAddress: strings.ToLower(bid.CollectionAddress), TokenID: bid.TokenId}, ChainName: chainName}] = bid
					continue
				}
				if bid.Price.GreaterThan(order.Price) {
					itemsBestBids[dao.MultiChainItemInfo{ItemInfo: types.ItemInfo{CollectionAddress: strings.ToLower(bid.CollectionAddress), TokenID: bid.TokenId}, ChainName: chainName}] = bid
				}
			}
		}(chain, items)
	}
	wg.Wait()
	if queryErr != nil {
		return nil, errors.Wrap(queryErr, "failed on query items best bids")
	}

	collections, err := svcCtx.Dao.QueryMultiChainCollectionsInfo(ctx, collectionAddrs)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query collections info")
	}

	collectionInfos := make(map[string]multi.Collection)
	for _, collection := range collections {
		collectionInfos[strings.ToLower(collection.Address)] = collection
	}

	listings, err := svcCtx.Dao.QueryMultiChainUserItemsExpireListInfo(ctx, userAddrs, itemInfos)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query item list info")
	}

	listingInfos := make(map[string]*dao.CollectionItem)
	for _, listing := range listings {
		listingInfos[strings.ToLower(listing.CollectionAddress+listing.TokenId)] = listing
	}

	var itemPrice []dao.MultiChainItemPriceInfo
	for _, item := range listingInfos {
		if item.Listing {
			chainName, ok := chainIDToChainName[item.ChainId]
			if !ok || strings.TrimSpace(chainName) == "" {
				continue
			}
			itemPrice = append(itemPrice, dao.MultiChainItemPriceInfo{
				ItemPriceInfo: types.ItemPriceInfo{
					CollectionAddress: item.CollectionAddress,
					TokenID:           item.TokenId,
					Maker:             item.Owner,
					Price:             item.ListPrice,
					OrderStatus:       item.OrderStatus,
				},
				ChainName: chainName,
			})
		}
	}

	orderIds := make(map[string]multi.Order)
	if len(itemPrice) > 0 {
		orders, err := svcCtx.Dao.QueryMultiChainListingInfo(ctx, itemPrice)
		if err != nil {
			return nil, errors.Wrap(err, "failed on query item order id")
		}

		for _, order := range orders {
			orderIds[strings.ToLower(order.CollectionAddress+order.TokenId)] = order
		}

	}

	itemImages, err := svcCtx.Dao.QueryMultiChainCollectionsItemsImage(ctx, itemInfos)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query item image info")
	}

	itemExternals := make(map[string]multi.ItemExternal)
	for _, item := range itemImages {
		itemExternals[strings.ToLower(item.CollectionAddress+item.TokenId)] = item
	}

	for i := 0; i < len(items); i++ {
		var resultlisting types.Listing
		listing, ok := listingInfos[strings.ToLower(items[i].CollectionAddress+items[i].TokenID)]
		if ok {
			resultlisting.ListPrice = listing.ListPrice
			resultlisting.MarketplaceID = listing.MarketID
		} else {
			count--
			continue
		}

		resultlisting.ChainID = items[i].ChainID
		resultlisting.CollectionAddress = items[i].CollectionAddress
		resultlisting.TokenID = items[i].TokenID
		resultlisting.LastCostPrice = itemLastCost[dao.MultiChainItemInfo{
			ItemInfo: types.ItemInfo{
				CollectionAddress: items[i].CollectionAddress,
				TokenID:           items[i].TokenID,
			},
			ChainName: chainIDToChainName[items[i].ChainID],
		}]
		bidOrder, ok := itemsBestBids[dao.MultiChainItemInfo{ItemInfo: types.ItemInfo{CollectionAddress: strings.ToLower(items[i].CollectionAddress), TokenID: items[i].TokenID}, ChainName: chainIDToChainName[items[i].ChainID]}]
		if ok {
			if bidOrder.Price.GreaterThan(collectionBestBids[types.MultichainCollection{
				CollectionAddress: strings.ToLower(items[i].CollectionAddress),
				Chain:             chainIDToChainName[items[i].ChainID],
			}].Price) {
				resultlisting.BidOrderID = bidOrder.OrderID
				resultlisting.BidExpireTime = bidOrder.ExpireTime
				resultlisting.BidPrice = bidOrder.Price
				resultlisting.BidTime = bidOrder.EventTime
				resultlisting.BidSalt = bidOrder.Salt
				resultlisting.BidMaker = bidOrder.Maker
				resultlisting.BidType = getBidType(bidOrder.OrderType)
				resultlisting.BidSize = bidOrder.Size
				resultlisting.BidUnfilled = bidOrder.QuantityRemaining
			}
		} else {
			bidOrder, ok := collectionBestBids[types.MultichainCollection{
				CollectionAddress: strings.ToLower(items[i].CollectionAddress),
				Chain:             chainIDToChainName[items[i].ChainID],
			}]

			if ok {
				resultlisting.BidOrderID = bidOrder.OrderID
				resultlisting.BidExpireTime = bidOrder.ExpireTime
				resultlisting.BidPrice = bidOrder.Price
				resultlisting.BidTime = bidOrder.EventTime
				resultlisting.BidSalt = bidOrder.Salt
				resultlisting.BidMaker = bidOrder.Maker
				resultlisting.BidType = getBidType(bidOrder.OrderType)
				resultlisting.BidSize = bidOrder.Size
				resultlisting.BidUnfilled = bidOrder.QuantityRemaining
			}
		}
		collection, ok := collectionInfos[strings.ToLower(items[i].CollectionAddress)]
		if ok {
			resultlisting.CollectionName = collection.Name
			if resultlisting.Name == "" {
				resultlisting.Name = fmt.Sprintf("%s #%s", collection.Name, items[i].TokenID)
			}
			resultlisting.FloorPrice = collection.FloorPrice
		}

		order, ok := orderIds[strings.ToLower(items[i].CollectionAddress+items[i].TokenID)]
		if ok {
			resultlisting.ListOrderID = order.OrderID
			resultlisting.ListExpireTime = order.ExpireTime
			resultlisting.ListMaker = order.Maker
			resultlisting.ListSalt = order.Salt
		}

		image, ok := itemExternals[strings.ToLower(items[i].CollectionAddress+items[i].TokenID)]
		if ok {
			if image.IsUploadedOss {
				resultlisting.ImageURI = image.OssUri // svcCtx.ImageMgr.GetSmallSizeImageUrl(image.OssUri)
			} else {
				resultlisting.ImageURI = image.ImageUri // svcCtx.ImageMgr.GetSmallSizeImageUrl(image.ImageUri)
			}
		}
		result = append(result, resultlisting)
	}

	return &types.UserListingsResp{
		Count:  count,
		Result: result,
	}, nil
}

type multiOrder struct {
	multi.Order
	chainID   int
	chainName string
}

func GetMultiChainUserBids(ctx context.Context, svcCtx *svc.ServerCtx, chainID []int, chainNames []string, userAddrs []string, contractAddrs []string, page, pageSize int) (*types.UserBidsResp, error) {
	var totalBids []multiOrder
	for i, chain := range chainNames {
		orders, err := svcCtx.Dao.QueryUserBids(ctx, chain, userAddrs, contractAddrs)
		if err != nil {
			return nil, errors.Wrap(err, "failed on get user bids info")
		}

		var tmpBids []multiOrder
		for j := 0; j < len(orders); j++ {
			tmpBids = append(tmpBids, multiOrder{
				Order:     orders[j],
				chainID:   chainID[i],
				chainName: chain,
			})
		}
		totalBids = append(totalBids, tmpBids...)
	}

	bidsMap := make(map[string]types.UserBid)
	bidCollections := make(map[string][]string)
	for _, bid := range totalBids {
		if collections, ok := bidCollections[bid.chainName]; ok {
			bidCollections[bid.chainName] = append(collections, strings.ToLower(bid.CollectionAddress))
		} else {
			bidCollections[bid.chainName] = []string{strings.ToLower(bid.CollectionAddress)}
		}

		key := strings.ToLower(bid.CollectionAddress) + bid.TokenId + bid.Price.String() + fmt.Sprintf("%d", bid.MarketplaceId) + fmt.Sprintf("%d", bid.ExpireTime) + fmt.Sprintf("%d", bid.OrderType)
		userBid, ok := bidsMap[key]
		if !ok {
			bidsMap[key] = types.UserBid{
				ChainID:           bid.chainID,
				CollectionAddress: strings.ToLower(bid.CollectionAddress),
				TokenID:           bid.TokenId,
				BidPrice:          bid.Price,
				MarketplaceID:     bid.MarketplaceId,
				ExpireTime:        bid.ExpireTime,
				BidType:           getBidType(bid.OrderType),
				OrderSize:         bid.QuantityRemaining,
				BidInfos: []types.BidInfo{
					{
						BidOrderID:    bid.OrderID,
						BidTime:       bid.EventTime,
						BidExpireTime: bid.ExpireTime,
						BidPrice:      bid.Price,
						BidSalt:       bid.Salt,
						BidSize:       bid.Size,
						BidUnfilled:   bid.QuantityRemaining,
					},
				},
			}
			continue
		}

		userBid.OrderSize += bid.QuantityRemaining
		userBid.BidInfos = append(userBid.BidInfos, types.BidInfo{
			BidOrderID:    bid.OrderID,
			BidTime:       bid.EventTime,
			BidExpireTime: bid.ExpireTime,
			BidPrice:      bid.Price,
			BidSalt:       bid.Salt,
			BidSize:       bid.Size,
			BidUnfilled:   bid.QuantityRemaining,
		})
		bidsMap[key] = userBid
	}

	collectionInfos := make(map[string]multi.Collection)
	for chain, collections := range bidCollections {
		cs, err := svcCtx.Dao.QueryCollectionsInfo(ctx, chain, removeRepeatedElement(collections))
		if err != nil {
			return nil, errors.Wrap(err, "failed on get collections info")
		}

		for _, c := range cs {
			collectionInfos[fmt.Sprintf("%d:%s", c.ChainId, strings.ToLower(c.Address))] = c
		}
	}

	var results []types.UserBid
	for _, userBid := range bidsMap {
		if c, ok := collectionInfos[fmt.Sprintf("%d:%s", userBid.ChainID, strings.ToLower(userBid.CollectionAddress))]; ok {
			userBid.CollectionName = c.Name
			userBid.ImageURI = c.ImageUri
		}

		results = append(results, userBid)
	}

	// Sort the bids slice based on the Volume in descending order
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].ExpireTime > (results[j].ExpireTime)
	})

	return &types.UserBidsResp{
		Count:  len(bidsMap),
		Result: results,
	}, nil
}

func removeRepeatedElement(arr []string) (newArr []string) {
	newArr = make([]string, 0)
	for i := 0; i < len(arr); i++ {
		repeat := false
		for j := i + 1; j < len(arr); j++ {
			if arr[i] == arr[j] {
				repeat = true
				break
			}
		}
		if !repeat && arr[i] != "" {
			newArr = append(newArr, arr[i])
		}
	}
	return
}
