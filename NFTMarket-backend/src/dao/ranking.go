package dao

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ProjectsTask/EasySwapBase/logger/xzap"
	"github.com/go-redis/redis"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type CollectionTrade struct {
	ContractAddress string          `json:"contract_address"`
	ItemCount       int64           `json:"item_count"`
	Volume          decimal.Decimal `json:"volume"`
	VolumeChange    int             `json:"volume_change"`
	PreFloorPrice   decimal.Decimal `json:"pre_floor_price"`
	FloorChange     int             `json:"floor_change"`
}

func GenRankingKey(project, chain string, period int) string {
	return fmt.Sprintf("cache:%s:%s:ranking:volume:%d", strings.ToLower(project), strings.ToLower(chain), period)
}

type periodEpochMap map[string]int

var periodToEpoch = periodEpochMap{
	"15m": 3,
	"1h":  12,
	"6h":  72,
	"24h": 288,
	"1d":  288,
	"7d":  2016,
	"30d": 8640,
}

func (d *Dao) QueryCollectionTradeInfo(project, chain, period string) ([]CollectionTrade, error) {
	epoch, ok := periodToEpoch[period]
	if !ok {
		return nil, errors.Errorf("invalid period: %s", period)
	}

	key := GenRankingKey(project, chain, epoch)
	cacheInfo, err := d.KvStore.Get(key)
	if err != nil {
		if err == redis.Nil {
			return []CollectionTrade{}, nil
		}
		return nil, errors.Wrap(err, "failed on get cache trending ranking info")
	}

	if cacheInfo == "" {
		return []CollectionTrade{}, nil
	}

	var tradeInfos []CollectionTrade
	if err := json.Unmarshal([]byte(cacheInfo), &tradeInfos); err != nil {
		xzap.WithContext(d.ctx).Error("failed on unmarshal global ranking info", zap.String("cacheInfo", cacheInfo), zap.Error(err))
		return nil, errors.Wrap(err, "failed on unmarshal global ranking info")
	}

	return tradeInfos, nil
}
