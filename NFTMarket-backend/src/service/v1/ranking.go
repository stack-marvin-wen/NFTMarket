package service

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/ProjectsTask/EasySwapBase/errcode"
	"github.com/ProjectsTask/EasySwapBase/logger/xzap"
	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/multi"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"github.com/ProjectsTask/EasySwapBackend/src/dao"
	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

const MinuteSeconds = 60
const HourSeconds = 60 * 60
const DaySeconds = 3600 * 24

func GetTopRanking(ctx context.Context, svcCtx *svc.ServerCtx, chain string, period string, limit int64) ([]*types.CollectionRankingInfo, error) {
	tradeInfos, err := svcCtx.Dao.QueryCollectionTradeInfo(svcCtx.C.ProjectCfg.Name, chain, period)
	if err != nil {
		xzap.WithContext(ctx).Error("failed on get collection trade info", zap.Error(err))
		//return nil, errcode.NewCustomErr("cache error")
	}

	collectionTradeMap := make(map[string]dao.CollectionTrade)
	for _, tradeInfo := range tradeInfos {
		collectionTradeMap[strings.ToLower(tradeInfo.ContractAddress)] = tradeInfo
	}

	periodTime := map[string]int64{
		"15m": MinuteSeconds * 15,
		"1h":  HourSeconds,
		"6h":  HourSeconds * 6,
		"1d":  DaySeconds,
		"7d":  DaySeconds * 7,
		"30d": DaySeconds * 30,
	}
	collectionFloorChange, err := svcCtx.Dao.QueryCollectionFloorChange(chain, periodTime[period])
	if err != nil {
		xzap.WithContext(ctx).Error("failed on get collection floor change", zap.Error(err))
	}

	var wg sync.WaitGroup
	var queryErr error

	collectionSells := make(map[string]multi.Collection)
	wg.Add(1)
	go func() {
		defer wg.Done()
		sellInfos, err := svcCtx.Dao.QueryCollectionsSellPrice(ctx, chain)
		if err != nil {
			xzap.WithContext(ctx).Error("failed on get all collections info", zap.Error(err))
			queryErr = errcode.NewCustomErr("failed on get all collections info")
			return
		}
		for _, sell := range sellInfos {
			collectionSells[strings.ToLower(sell.Address)] = sell
		}
	}()

	// get collection info
	var allCollections []multi.Collection
	wg.Add(1)
	go func() {
		defer wg.Done()
		allCollections, err = svcCtx.Dao.QueryAllCollectionInfo(ctx, chain)
		if err != nil {
			xzap.WithContext(ctx).Error("failed on get all collections info", zap.Error(err))
			queryErr = errcode.NewCustomErr("failed on get all collections info")
			return
		}
	}()

	wg.Wait()

	if queryErr != nil {
		return nil, queryErr
	}

	var respInfos []*types.CollectionRankingInfo
	for _, collection := range allCollections {
		var priceChange float64
		var volume decimal.Decimal
		var sellPrice decimal.Decimal
		var sales int64

		tradeInfo, ok := collectionTradeMap[strings.ToLower(collection.Address)]
		if ok {
			priceChange = collectionFloorChange[strings.ToLower(collection.Address)]
			volume = tradeInfo.Volume
			sales = tradeInfo.ItemCount
		}
		sellInfo, ok := collectionSells[strings.ToLower(collection.Address)]
		if ok {
			sellPrice = sellInfo.SalePrice
		}

		var listAmount int
		listed, err := svcCtx.Dao.QueryCollectionsListed(ctx, chain, []string{collection.Address})
		if err != nil {
			xzap.WithContext(ctx).Error("failed on query collection listed", zap.Error(err))
		} else {
			listAmount = listed[0].Count
		}
		respInfos = append(respInfos, &types.CollectionRankingInfo{
			Name:        collection.Name,
			Address:     collection.Address,
			ImageUri:    collection.ImageUri, // svcCtx.ImageMgr.GetFileUrl(collection.ImageUri),
			Banner:      firstNonEmpty(collection.Banner, collection.BannerUri),
			BannerURI:   firstNonEmpty(collection.BannerUri, collection.Banner),
			FloorPrice:  collection.FloorPrice.String(),
			FloorChange: strconv.FormatFloat(priceChange, 'f', 4, 32),
			SellPrice:   sellPrice.String(),
			Volume:      volume,
			ItemSold:    sales,
			ItemNum:     collection.ItemAmount,
			ItemOwner:   collection.OwnerAmount,
			ListAmount:  listAmount,
			ChainID:     collection.ChainId,
			Owner:       collection.Creator,
		})
	}

	if limit < int64(len(respInfos)) {
		respInfos = respInfos[:limit]
	}

	return respInfos, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
