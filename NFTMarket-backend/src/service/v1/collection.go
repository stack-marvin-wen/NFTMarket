package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ProjectsTask/EasySwapBase/errcode"
	"github.com/ProjectsTask/EasySwapBase/evm/eip"
	"github.com/ProjectsTask/EasySwapBase/logger/xzap"
	"github.com/ProjectsTask/EasySwapBase/ordermanager"
	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/multi"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/ProjectsTask/EasySwapBackend/src/dao"
	"github.com/ProjectsTask/EasySwapBackend/src/service/mq"
	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

func GetBids(ctx context.Context, svcCtx *svc.ServerCtx, chain string, collectionAddr string, page, pageSize int) (*types.CollectionBidsResp, error) {
	bids, count, err := svcCtx.Dao.QueryCollectionBids(ctx, chain, collectionAddr, page, pageSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed on get item info")
	}

	return &types.CollectionBidsResp{
		Result: bids,
		Count:  count,
	}, nil
}

func GetItems(ctx context.Context, svcCtx *svc.ServerCtx, chain string, filter types.CollectionItemFilterParams, collectionAddr string) (*types.NFTListingInfoResp, error) {
	items, count, err := svcCtx.Dao.QueryCollectionItemBySupply(ctx, chain, filter, collectionAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed on get item info")
	}

	var ItemIds []string
	var ItemOwners []string
	for _, item := range items {
		if item.TokenId != "" {
			ItemIds = append(ItemIds, item.TokenId)
		}
		if item.Owner != "" {
			ItemOwners = append(ItemOwners, item.Owner)
		}
	}

	var queryErr error
	var wg sync.WaitGroup

	ItemsExternal := make(map[string]multi.ItemExternal)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if len(ItemIds) != 0 {
			items, err := svcCtx.Dao.QueryCollectionItemsImage(ctx, chain, collectionAddr, ItemIds)
			if err != nil {
				queryErr = errors.Wrap(err, "failed on get items image info")
				return
			}

			for _, item := range items {
				ItemsExternal[strings.ToLower(item.TokenId)] = item
			}
		}

	}()

	userItemCount := make(map[string]int64)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if len(ItemIds) != 0 {
			itemCount, err := svcCtx.Dao.QueryUsersItemCount(ctx, chain, collectionAddr, ItemOwners)
			if err != nil {
				queryErr = errors.Wrap(err, "failed on get items image info")
				return
			}

			for _, v := range itemCount {
				userItemCount[strings.ToLower(v.Owner)] = v.Counts
			}
		}
	}()

	lastSales := make(map[string]decimal.Decimal)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if len(ItemIds) != 0 {
			lastSale, err := svcCtx.Dao.QueryLastSalePrice(ctx, chain, collectionAddr, ItemIds)
			if err != nil {
				queryErr = errors.Wrap(err, "failed on get items last sale info")
				return
			}

			for _, v := range lastSale {
				lastSales[strings.ToLower(v.TokenId)] = v.Price
			}
		}
	}()

	wg.Wait()
	if queryErr != nil {
		return nil, errors.Wrap(queryErr, "failed on get items info")
	}

	var respItems []*types.NFTListingInfo
	for _, item := range items {
		nameStr := item.Name
		if nameStr == "" {
			nameStr = fmt.Sprintf("#%s", item.TokenId)
		}

		respItem := &types.NFTListingInfo{
			Name:              nameStr,
			CollectionAddress: item.CollectionAddress,
			TokenID:           item.TokenId,
			OwnerAddress:      item.Owner,
			ListPrice:         item.ListPrice,
			MarketID:          item.MarketID,
		}

		itemExternal, ok := ItemsExternal[strings.ToLower(item.TokenId)]
		if ok {
			if itemExternal.IsUploadedOss {
				respItem.ImageURI = itemExternal.OssUri // svcCtx.ImageMgr.GetSmallSizeImageUrl(itemExternal.OssUri)
			} else {
				respItem.ImageURI = itemExternal.ImageUri // svcCtx.ImageMgr.GetSmallSizeImageUrl(itemExternal.ImageUri)
			}
			if len(itemExternal.VideoUri) > 0 {
				respItem.VideoType = itemExternal.VideoType
				if itemExternal.IsVideoUploaded {
					respItem.VideoURI = itemExternal.VideoOssUri // svcCtx.ImageMgr.GetFileUrl(itemExternal.VideoOssUri)
				} else {
					respItem.VideoURI = itemExternal.VideoUri // svcCtx.ImageMgr.GetFileUrl(itemExternal.VideoUri)
				}
			}
		}

		count, ok := userItemCount[strings.ToLower(item.Owner)]
		if ok {
			respItem.OwnerOwnedAmount = count
		}

		price, ok := lastSales[strings.ToLower(item.TokenId)]
		if ok {
			respItem.LastSellPrice = price
		}

		respItems = append(respItems, respItem)
	}

	return &types.NFTListingInfoResp{
		Result: respItems,
		Count:  count,
	}, nil
}

func GetItem(ctx context.Context, svcCtx *svc.ServerCtx, chain string, chainID int, collectionAddr, tokenID string) (*types.ItemDetailInfoResp, error) {
	var queryErr error
	var wg sync.WaitGroup
	var itemListInfo *dao.CollectionItem
	var collection *multi.Collection
	wg.Add(1)
	go func() {
		defer wg.Done()
		collection, queryErr = svcCtx.Dao.QueryCollectionInfo(ctx, chain, collectionAddr)
		if queryErr != nil {
			return
		}
	}()

	var item *multi.Item
	wg.Add(1)
	go func() {
		defer wg.Done()
		item, queryErr = svcCtx.Dao.QueryItemInfo(ctx, chain, collectionAddr, tokenID)
		if queryErr != nil {
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		itemListInfo, queryErr = svcCtx.Dao.QueryItemListInfo(ctx, chain, collectionAddr, tokenID)
		if queryErr != nil {
			return
		}
	}()
	ItemExternals := make(map[string]multi.ItemExternal)
	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := svcCtx.Dao.QueryCollectionItemsImage(ctx, chain, collectionAddr, []string{tokenID})
		if err != nil {
			queryErr = errors.Wrap(err, "failed on get items image info")
			return
		}

		for _, item := range items {
			ItemExternals[strings.ToLower(item.TokenId)] = item
		}
	}()

	lastSales := make(map[string]decimal.Decimal)
	wg.Add(1)
	go func() {
		defer wg.Done()
		lastSale, err := svcCtx.Dao.QueryLastSalePrice(ctx, chain, collectionAddr, []string{tokenID})
		if err != nil {
			queryErr = errors.Wrap(err, "failed on get items last sale info")
			return
		}

		for _, v := range lastSale {
			lastSales[strings.ToLower(v.TokenId)] = v.Price
		}
	}()

	bestBids := make(map[string]multi.Order)
	wg.Add(1)
	go func() {
		defer wg.Done()
		bids, err := svcCtx.Dao.QueryBestBids(ctx, chain, "", collectionAddr, []string{tokenID})
		if err != nil {
			queryErr = errors.Wrap(err, "failed on get items last sale info")
			return
		}

		for _, bid := range bids {
			order, ok := bestBids[strings.ToLower(bid.TokenId)]
			if !ok {
				bestBids[strings.ToLower(bid.TokenId)] = bid
				continue
			}
			if bid.Price.GreaterThan(order.Price) {
				bestBids[strings.ToLower(bid.TokenId)] = bid
			}
		}
	}()

	var collectionBestBid multi.Order
	wg.Add(1)
	go func() {
		defer wg.Done()
		bid, err := svcCtx.Dao.QueryCollectionBestBid(ctx, chain, "", collectionAddr)
		if err != nil {
			queryErr = errors.Wrap(err, "failed on get items last sale info")
			return
		}
		collectionBestBid = bid
	}()

	wg.Wait()
	if queryErr != nil {
		return nil, errors.Wrap(queryErr, "failed on get items info")
	}

	var itemDetail types.ItemDetailInfo
	itemDetail.ChainID = chainID
	if item != nil {
		itemDetail.Name = item.Name
		itemDetail.CollectionAddress = item.CollectionAddress
		itemDetail.TokenID = item.TokenId
		itemDetail.OwnerAddress = item.Owner
		itemDetail.BidOrderID = collectionBestBid.OrderID
		itemDetail.BidExpireTime = collectionBestBid.ExpireTime
		itemDetail.BidPrice = collectionBestBid.Price
		itemDetail.BidTime = collectionBestBid.EventTime
		itemDetail.BidSalt = collectionBestBid.Salt
		itemDetail.BidMaker = collectionBestBid.Maker
		itemDetail.BidType = getBidType(collectionBestBid.OrderType)
		itemDetail.BidSize = collectionBestBid.Size
		itemDetail.BidUnfilled = collectionBestBid.QuantityRemaining
	}

	bidOrder, ok := bestBids[strings.ToLower(item.TokenId)]
	if ok {
		if bidOrder.Price.GreaterThan(collectionBestBid.Price) {
			itemDetail.BidOrderID = bidOrder.OrderID
			itemDetail.BidExpireTime = bidOrder.ExpireTime
			itemDetail.BidPrice = bidOrder.Price
			itemDetail.BidTime = bidOrder.EventTime
			itemDetail.BidSalt = bidOrder.Salt
			itemDetail.BidMaker = bidOrder.Maker
			itemDetail.BidType = getBidType(bidOrder.OrderType)
			itemDetail.BidSize = bidOrder.Size
			itemDetail.BidUnfilled = bidOrder.QuantityRemaining
		}
	}

	if itemListInfo != nil {
		itemDetail.ListPrice = itemListInfo.ListPrice
		itemDetail.MarketplaceID = itemListInfo.MarketID
		itemDetail.ListOrderID = itemListInfo.OrderID
		itemDetail.ListTime = itemListInfo.ListTime
		itemDetail.ListExpireTime = itemListInfo.ListExpireTime
		itemDetail.ListSalt = itemListInfo.ListSalt
		itemDetail.ListMaker = itemListInfo.ListMaker
	}

	if collection != nil {
		itemDetail.CollectionName = collection.Name
		itemDetail.FloorPrice = collection.FloorPrice
		itemDetail.CollectionImageURI = collection.ImageUri
		if itemDetail.Name == "" {
			itemDetail.Name = fmt.Sprintf("%s #%s", collection.Name, tokenID)
		}
	}
	price, ok := lastSales[strings.ToLower(tokenID)]
	if ok {
		itemDetail.LastSellPrice = price
	}

	itemExternal, ok := ItemExternals[strings.ToLower(tokenID)]
	if ok {
		itemDetail.ImageURI = itemExternal.ImageUri // svcCtx.ImageMgr.GetFileUrl(itemExternal.ImageUri)
		if itemExternal.IsUploadedOss {
			itemDetail.ImageURI = itemExternal.OssUri // svcCtx.ImageMgr.GetFileUrl(itemExternal.OssUri)
		}
		if len(itemExternal.VideoUri) > 0 {
			itemDetail.VideoType = itemExternal.VideoType
			if itemExternal.IsVideoUploaded {
				itemDetail.VideoURI = itemExternal.VideoOssUri // svcCtx.ImageMgr.GetFileUrl(itemExternal.VideoOssUri)
			} else {
				itemDetail.VideoURI = itemExternal.VideoUri // svcCtx.ImageMgr.GetFileUrl(itemExternal.VideoUri)
			}
		}
	}

	return &types.ItemDetailInfoResp{
		Result: itemDetail,
	}, nil
}

func GetItemTopTraitPrice(ctx context.Context, svcCtx *svc.ServerCtx, chain, collectionAddr string, tokenIDs []string) (*types.ItemTopTraitResp, error) {
	traitsPrice, err := svcCtx.Dao.QueryTraitsPrice(ctx, chain, collectionAddr, tokenIDs)
	if err != nil {
		return nil, errors.Wrap(err, "failed on calc top trait")
	}

	if len(traitsPrice) == 0 {
		return &types.ItemTopTraitResp{
			Result: []types.TraitPrice{},
		}, nil
	}

	traitsPrices := make(map[string]decimal.Decimal)
	for _, traitPrice := range traitsPrice {
		traitsPrices[strings.ToLower(fmt.Sprintf("%s:%s", traitPrice.Trait, traitPrice.TraitValue))] = traitPrice.Price
	}
	traits, err := svcCtx.Dao.QueryItemsTraits(ctx, chain, collectionAddr, tokenIDs)
	if err != nil {
		return nil, errors.Wrap(err, "failed on query items trait")
	}

	topTraits := make(map[string]types.TraitPrice)
	for _, trait := range traits {
		key := strings.ToLower(fmt.Sprintf("%s:%s", trait.Trait, trait.TraitValue))
		price, ok := traitsPrices[key]
		if ok {
			topPrice, ok := topTraits[trait.TokenId]
			if ok {
				if price.LessThanOrEqual(topPrice.Price) {
					continue
				}
			}

			topTraits[trait.TokenId] = types.TraitPrice{
				CollectionAddress: collectionAddr,
				TokenID:           trait.TokenId,
				Trait:             trait.Trait,
				TraitValue:        trait.TraitValue,
				Price:             price,
			}
		}
	}

	var results []types.TraitPrice
	for _, topTrait := range topTraits {
		results = append(results, topTrait)
	}

	return &types.ItemTopTraitResp{
		Result: results,
	}, nil
}

func GetHistorySalesPrice(ctx context.Context, svcCtx *svc.ServerCtx, chain, collectionAddr, duration string) ([]types.HistorySalesPriceInfo, error) {
	var durationTimeStamp int64
	if duration == "24h" {
		durationTimeStamp = 24 * 60 * 60
	} else if duration == "7d" {
		durationTimeStamp = 7 * 24 * 60 * 60
	} else if duration == "30d" {
		durationTimeStamp = 30 * 24 * 60 * 60
	} else {
		return nil, errors.New("only support 24h/7d/30d")
	}

	historySalesPriceInfo, err := svcCtx.Dao.QueryHistorySalesPriceInfo(ctx, chain, collectionAddr, durationTimeStamp)
	if err != nil {
		return nil, errors.Wrap(err, "failed on get history sales price info")
	}

	res := make([]types.HistorySalesPriceInfo, len(historySalesPriceInfo))

	for i, ele := range historySalesPriceInfo {
		res[i] = types.HistorySalesPriceInfo{
			Price:     ele.Price,
			TokenID:   ele.TokenId,
			TimeStamp: ele.EventTime,
		}
	}

	return res, nil
}

func GetItemOwner(ctx context.Context, svcCtx *svc.ServerCtx, chainID int64, chain, collectionAddr, tokenID string) (*types.ItemOwner, error) {
	address, err := svcCtx.NodeSrvs[chainID].FetchNftOwner(collectionAddr, tokenID)
	if err != nil {
		xzap.WithContext(ctx).Error("failed on fetch nft owner onchain", zap.Error(err))
		return nil, errcode.ErrUnexpected
	}

	owner, err := eip.ToCheckSumAddress(address.String())
	if err != nil {
		xzap.WithContext(ctx).Error("invalid address", zap.Error(err), zap.String("address", address.String()))
		return nil, errcode.ErrUnexpected
	}

	if err := svcCtx.Dao.UpdateItemOwner(ctx, chain, collectionAddr, tokenID, owner); err != nil {
		xzap.WithContext(ctx).Error("failed on update item owner", zap.Error(err), zap.String("address", address.String()))
	}

	return &types.ItemOwner{
		CollectionAddress: collectionAddr,
		TokenID:           tokenID,
		Owner:             owner,
	}, nil
}

func GetItemTraits(ctx context.Context, svcCtx *svc.ServerCtx, chain, collectionAddr, tokenID string) ([]types.TraitInfo, error) {
	var traitInfos []types.TraitInfo
	var itemTraits []multi.ItemTrait
	var collection *multi.Collection
	var traitCounts []types.TraitCount
	var queryErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		itemTraits, queryErr = svcCtx.Dao.QueryItemTraits(ctx, chain, collectionAddr, tokenID)
		if queryErr != nil {
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		traitCounts, queryErr = svcCtx.Dao.QueryCollectionTraits(ctx, chain, collectionAddr)
		if queryErr != nil {
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		collection, queryErr = svcCtx.Dao.QueryCollectionInfo(ctx, chain, collectionAddr)
		if queryErr != nil {
			return
		}
	}()

	wg.Wait()
	if queryErr != nil {
		return nil, queryErr
	}

	if len(itemTraits) == 0 {
		return traitInfos, nil
	}

	traitCountMap := make(map[string]int64)
	for _, trait := range traitCounts {
		traitCountMap[fmt.Sprintf("%s-%s", trait.Trait, trait.TraitValue)] = trait.Count
	}

	for _, trait := range itemTraits {
		key := fmt.Sprintf("%s-%s", trait.Trait, trait.TraitValue)
		if count, ok := traitCountMap[key]; ok {
			traitPercent := 0.0
			if collection.ItemAmount != 0 {
				traitPercent = decimal.NewFromInt(count).DivRound(decimal.NewFromInt(collection.ItemAmount), 4).Mul(decimal.NewFromInt(100)).InexactFloat64()
			}
			traitInfos = append(traitInfos, types.TraitInfo{
				Trait:        trait.Trait,
				TraitValue:   trait.TraitValue,
				TraitAmount:  count,
				TraitPercent: traitPercent,
			})
		}
	}

	return traitInfos, nil
}

func GetCollectionDetail(ctx context.Context, svcCtx *svc.ServerCtx, chain string, collectionAddr string) (*types.CollectionDetailResp, error) {
	collection, err := svcCtx.Dao.QueryCollectionInfo(ctx, chain, collectionAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed on get collection info")
	}

	tradeInfos, err := svcCtx.Dao.QueryCollectionTradeInfo(svcCtx.C.ProjectCfg.Name, chain, "1d")
	if err != nil {
		xzap.WithContext(ctx).Error("failed on get collection trade info", zap.Error(err))
		//return nil, errcode.NewCustomErr("cache error")
	}
	listed, err := svcCtx.Dao.QueryListedAmount(ctx, chain, collectionAddr)
	if err != nil {
		xzap.WithContext(ctx).Error("failed on get listed count", zap.Error(err))
		//return nil, errcode.NewCustomErr("cache error")
	} else {
		if err := svcCtx.Dao.CacheCollectionsListed(ctx, chain, collectionAddr, int(listed)); err != nil {
			xzap.WithContext(ctx).Error("failed on cache collection listed", zap.Error(err))
		}
	}

	floorPrice, err := svcCtx.Dao.QueryFloorPrice(ctx, chain, collectionAddr)
	if err != nil {
		xzap.WithContext(ctx).Error("failed on get floor price", zap.Error(err))
	}

	collectionSell, err := svcCtx.Dao.QueryCollectionSellPrice(ctx, chain, collectionAddr)
	if err != nil {
		xzap.WithContext(ctx).Error("failed on get floor price", zap.Error(err))
	}

	if !floorPrice.Equal(collection.FloorPrice) {
		if err := ordermanager.AddUpdatePriceEvent(svcCtx.KvStore, &ordermanager.TradeEvent{
			EventType:      ordermanager.UpdateCollection,
			CollectionAddr: collectionAddr,
			Price:          floorPrice,
		}, chain); err != nil {
			xzap.WithContext(ctx).Error("failed on update floor price", zap.Error(err))
		}
	}

	var volume24h decimal.Decimal
	var sold int64
	for _, tradeInfo := range tradeInfos {
		if strings.ToLower(tradeInfo.ContractAddress) == strings.ToLower(collectionAddr) {
			volume24h = tradeInfo.Volume
			sold = tradeInfo.ItemCount
		}
	}

	var allVol decimal.Decimal
	collectionVol, err := svcCtx.Dao.QueryCollectionAllVolume(svcCtx.C.ProjectCfg.Name, chain, collectionAddr)
	if err != nil {
		xzap.WithContext(ctx).Error("failed on query collection all volume", zap.Error(err))
	} else {
		allVol = collectionVol.Volume
	}

	detail := types.CollectionDetail{
		ImageUri:    collection.ImageUri, // svcCtx.ImageMgr.GetFileUrl(collection.ImageUri),
		Name:        collection.Name,
		Address:     collection.Address,
		ChainId:     collection.ChainId,
		FloorPrice:  floorPrice,
		SellPrice:   collectionSell.SalePrice.String(),
		VolumeTotal: allVol,
		Volume24h:   volume24h,
		Sold24h:     sold,
		ListAmount:  listed,
		TotalSupply: collection.ItemAmount,
		OwnerAmount: collection.OwnerAmount,
	}

	return &types.CollectionDetailResp{
		Result: detail,
	}, nil
}

// RefreshItemMetadata refresh item meta data.
func RefreshItemMetadata(ctx context.Context, svcCtx *svc.ServerCtx, chainName string, chainId int64, collectionAddress, tokenId string) error {
	if err := mq.AddSingleItemToRefreshMetadataQueue(svcCtx.KvStore, svcCtx.C.ProjectCfg.Name, chainName, chainId, collectionAddress, tokenId); err != nil {
		xzap.WithContext(ctx).Error("failed on add item to refresh queue", zap.Error(err), zap.String("collection address: ", collectionAddress), zap.String("item_id", tokenId))
		return errcode.ErrUnexpected
	}

	return nil

}

func GetItemImage(ctx context.Context, svcCtx *svc.ServerCtx, chain string, collectionAddress, tokenId string) (*types.ItemImage, error) {
	items, err := svcCtx.Dao.QueryCollectionItemsImage(ctx, chain, collectionAddress, []string{tokenId})
	if err != nil || len(items) == 0 {
		return nil, errors.Wrap(err, "failed on get item image")
	}
	var imageUri string
	if items[0].IsUploadedOss {
		imageUri = items[0].OssUri // svcCtx.ImageMgr.GetSmallSizeImageUrl(items[0].OssUri)
	} else {
		imageUri = items[0].ImageUri // svcCtx.ImageMgr.GetSmallSizeImageUrl(items[0].ImageUri)
	}

	return &types.ItemImage{
		CollectionAddress: collectionAddress,
		TokenID:           tokenId,
		ImageUri:          imageUri,
	}, nil
}

// MintNFT 安全的NFT铸造服务
func MintNFT(ctx context.Context, svcCtx *svc.ServerCtx, chain, collectionAddr, userAddr string, req *types.MintRequest) (*types.MintResult, error) {
	// 这里需要实现智能合约交互逻辑
	// 注意：实际的私钥应该从环境变量或安全的密钥管理系统中获取
	
	// 验证用户权限
	if !isAuthorizedMinter(userAddr) {
		return nil, errors.New("unauthorized minter")
	}
	
	// 验证请求参数
	if err := validateMintRequest(req); err != nil {
		return nil, err
	}
	
	// 这里应该调用区块链服务进行实际的铸造
	// 示例代码 - 实际实现需要根据您的区块链服务来调整
	result, err := performMint(ctx, svcCtx, chain, collectionAddr, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to mint NFT")
	}
	
	return result, nil
}

// isAuthorizedMinter 检查用户是否有铸造权限
func isAuthorizedMinter(userAddr string) bool {
	// 这里应该实现实际的权限检查逻辑
	// 例如检查用户是否在白名单中，或者是否是合约所有者
	
	// 临时实现 - 实际应用中需要更严格的权限控制
	authorizedMinters := map[string]bool{
		// 添加授权的铸造者地址
		"0x742d35Cc6634C0532925a3b8D0Ac9C4C0C0C0C0C": true, // 示例地址
	}
	
	return authorizedMinters[userAddr]
}

// validateMintRequest 验证铸造请求参数
func validateMintRequest(req *types.MintRequest) error {
	if req.ToAddress == "" {
		return errors.New("to_address is required")
	}
	
	if req.TokenURI == "" {
		return errors.New("token_uri is required")
	}
	
	// 验证地址格式
	if !isValidEthereumAddress(req.ToAddress) {
		return errors.New("invalid to_address format")
	}
	
	return nil
}

// isValidEthereumAddress 验证以太坊地址格式
func isValidEthereumAddress(address string) bool {
	// 简单的地址格式验证
	if len(address) != 42 {
		return false
	}
	
	if address[:2] != "0x" {
		return false
	}
	
	// 这里可以添加更严格的地址验证逻辑
	return true
}

// performMint 执行实际的铸造操作
func performMint(ctx context.Context, svcCtx *svc.ServerCtx, chain, collectionAddr string, req *types.MintRequest) (*types.MintResult, error) {
	// 重要提示：这里需要实现实际的区块链交互
	// 私钥应该从环境变量中安全获取，例如：
	// privateKey := os.Getenv("MINTER_PRIVATE_KEY")
	
	// 示例返回 - 实际实现需要调用区块链服务
	return &types.MintResult{
		TxHash:  "0x" + generateMockTxHash(), // 实际应该是真实的交易哈希
		TokenID: generateNextTokenID(),        // 实际应该是合约返回的token ID
		Status:  "pending",                    // 交易状态
	}, nil
}

// generateMockTxHash 生成模拟交易哈希（仅用于示例）
func generateMockTxHash() string {
	// 实际应用中这应该是真实的交易哈希
	return "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
}

// generateNextTokenID 生成下一个token ID（仅用于示例）
func generateNextTokenID() string {
	// 实际应用中这应该从合约或数据库中获取
	return "1001"
}
