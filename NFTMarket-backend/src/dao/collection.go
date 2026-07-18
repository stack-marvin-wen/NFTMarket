package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ProjectsTask/EasySwapBase/ordermanager"
	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/multi"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"

	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

const MaxBatchReadCollections = 500
const MaxRetries = 3
const QueryTimeout = time.Second * 30

var collectionFields = []string{"id", "chain_id", "token_standard", "name", "address", "image_uri", "banner_uri", "floor_price", "sale_price", "item_amount", "owner_amount", "creator"}

func (d *Dao) QueryHistorySalesPriceInfo(ctx context.Context, chain string, collectionAddr string, durationTimeStamp int64) ([]multi.Activity, error) {
	var historySalesInfo []multi.Activity
	now := time.Now().Unix()
	if err := d.DB.WithContext(ctx).Table(multi.ActivityTableName(chain)).
		Select("price", "token_id", "event_time").Where("activity_type = ? and collection_address = ? and event_time >= ? and event_time <= ?", multi.Sale, collectionAddr, now-durationTimeStamp, now).
		Find(&historySalesInfo).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get history sales info")
	}

	return historySalesInfo, nil
}

func (d *Dao) QueryAllCollectionInfo(ctx context.Context, chain string) ([]multi.Collection, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeout)
	defer cancel()

	tx := d.DB.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	cursor := int64(0)
	var allCollections []multi.Collection
	for {
		var collections []multi.Collection
		for i := 0; i < MaxRetries; i++ {
			err := tx.Table(multi.CollectionTableName(chain)).
				Select(collectionFields).
				Where("id > ?", cursor).
				Limit(MaxBatchReadCollections).
				Order("id asc").
				Scan(&collections).Error

			if err == nil {
				break
			}
			if i == MaxRetries-1 {
				tx.Rollback()
				return nil, errors.Wrap(err, "failed on get collections info")
			}
			time.Sleep(time.Duration(i+1) * time.Second)
		}

		allCollections = append(allCollections, collections...)
		if len(collections) < MaxBatchReadCollections {
			break
		}

		cursor = collections[len(collections)-1].Id
	}

	if err := tx.Commit().Error; err != nil {
		return nil, errors.Wrap(err, "failed to commit transaction")
	}
	return allCollections, nil
}

func (d *Dao) QueryCollectionInfo(ctx context.Context, chain string, collectionAddr string) (*multi.Collection, error) {
	var collection multi.Collection
	if err := d.DB.WithContext(ctx).Table(multi.CollectionTableName(chain)).
		Select(collectionDetailFields).Where("address = ?", collectionAddr).
		First(&collection).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get collection info")
	}

	return &collection, nil
}

func (d *Dao) QueryCollectionsInfo(ctx context.Context, chain string, collectionAddrs []string) ([]multi.Collection, error) {
	addrs := removeRepeatedElement(collectionAddrs)
	var collections []multi.Collection
	if err := d.DB.WithContext(ctx).Table(multi.CollectionTableName(chain)).
		Select(collectionDetailFields).Where("address in (?)", addrs).
		Scan(&collections).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get collection info")
	}

	return collections, nil
}

func (d *Dao) QueryMultiChainCollectionsInfo(ctx context.Context, collectionAddrs [][]string) ([]multi.Collection, error) {
	addrs := removeRepeatedElementArr(collectionAddrs)
	var collections []multi.Collection
	var collection multi.Collection
	for _, collectionAddr := range addrs {
		if err := d.DB.WithContext(ctx).Table(multi.CollectionTableName(collectionAddr[1])).
			Select(collectionDetailFields).Where("address = ?", collectionAddr[0]).
			Scan(&collection).Error; err != nil {
			return nil, errors.Wrap(err, "failed on get collection info")
		}
		collections = append(collections, collection)
	}

	return collections, nil
}

func (d *Dao) QueryMultiChainUserCollectionInfos(ctx context.Context, chainID []int, chainNames []string, userAddrs []string) ([]types.UserCollections, error) {
	var userCollections []types.UserCollections
	var userAddrsParam string
	for i, addr := range userAddrs {
		userAddrsParam += fmt.Sprintf(`'%s'`, addr)
		if i < len(userAddrs)-1 {
			userAddrsParam += ","
		}
	}

	sqlHead := "SELECT * FROM ("
	sqlTail := ") as combined ORDER BY combined.floor_price * CAST(combined.item_count AS DECIMAL) DESC"
	var sqlMids []string

	for _, chainName := range chainNames {
		//splice sqlMid
		sqlMid := "("
		sqlMid += "select "
		sqlMid += "gc.address as address, gc.name as name, gc.floor_price as floor_price, gc.chain_id as chain_id, gc.item_amount as item_amount, gc.symbol as symbol, gc.image_uri as image_uri, count(*) as item_count "
		sqlMid += fmt.Sprintf("from %s as gc ", multi.CollectionTableName(chainName))
		sqlMid += fmt.Sprintf("join %s as gi ", multi.ItemTableName(chainName))
		sqlMid += "on gc.address = gi.collection_address "
		sqlMid += fmt.Sprintf("where gi.owner in (%s) ", userAddrsParam)
		sqlMid += "group by gc.address"
		sqlMid += ")"

		//store into slice
		sqlMids = append(sqlMids, sqlMid)
	}

	sql := sqlHead
	for i := 0; i < len(sqlMids); i++ {
		if i != 0 {
			sql += " UNION ALL "
		}
		sql += sqlMids[i]
	}
	sql += sqlTail
	// execute sql
	if err := d.DB.WithContext(ctx).Raw(sql).Scan(&userCollections).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get user multi chain collection infos")
	}

	return userCollections, nil
}

func (d *Dao) QueryMultiChainUserItemInfos(ctx context.Context, chain []string, userAddrs []string, contractAddrs []string, page, pageSize int) ([]types.PortfolioItemInfo, int64, error) {
	var count int64
	var items []types.PortfolioItemInfo
	var userAddrsParam string
	for i, addr := range userAddrs {
		userAddrsParam += fmt.Sprintf(`'%s'`, addr)
		if i < len(userAddrs)-1 {
			userAddrsParam += ","
		}
	}

	sqlCntHead := "SELECT COUNT(*) FROM ("
	sqlHead := "SELECT * FROM ("
	sqlTail := fmt.Sprintf(") as combined ORDER BY combined.owned_time DESC LIMIT %d OFFSET %d", pageSize, page-1)
	var sqlMids []string

	for _, chainName := range chain {
		//splice sqlMid
		sqlMid := "("
		sqlMid += "select gi.chain_id as chain_id, gi.collection_address as collection_address, gi.token_id as token_id, gi.name as name, gi.owner as owner,0 as rarity_rank, sub.last_event_time as owned_time "
		sqlMid += fmt.Sprintf("from %s gi ", multi.ItemTableName(chainName))
		sqlMid += "left join "
		sqlMid += "(select sgi.collection_address, sgi.token_id, max(sga.event_time) as last_event_time "
		sqlMid += fmt.Sprintf("from %s sgi join %s sga ", multi.ItemTableName(chainName), multi.ActivityTableName(chainName))
		sqlMid += "on sgi.collection_address = sga.collection_address and sgi.token_id = sga.token_id "
		sqlMid += fmt.Sprintf("where sgi.owner in (%s) and sga.activity_type = %d ", userAddrsParam, multi.Sale)
		if len(contractAddrs) > 0 {
			sqlMid += fmt.Sprintf("and sgi.collection_address in ('%s'", contractAddrs[0])
			for i := 1; i < len(contractAddrs); i++ {
				sqlMid += fmt.Sprintf(",'%s'", contractAddrs[i])
			}
			sqlMid += ") "
		}
		sqlMid += "group by sgi.collection_address, sgi.token_id) sub "
		sqlMid += "on gi.collection_address = sub.collection_address and gi.token_id = sub.token_id "
		sqlMid += fmt.Sprintf("where gi.owner in (%s) ", userAddrsParam)
		if len(contractAddrs) > 0 {
			sqlMid += fmt.Sprintf("and gi.collection_address in ('%s'", contractAddrs[0])
			for i := 1; i < len(contractAddrs); i++ {
				sqlMid += fmt.Sprintf(",'%s'", contractAddrs[i])
			}
			sqlMid += ")"
		}
		sqlMid += ")"
		//store into slice
		sqlMids = append(sqlMids, sqlMid)
	}

	sqlCnt := sqlCntHead
	sql := sqlHead
	for i := 0; i < len(sqlMids); i++ {
		if i != 0 {
			sql += " UNION ALL "
			sqlCnt += " UNION ALL "
		}
		sql += sqlMids[i]
		sqlCnt += sqlMids[i]
	}
	sql += sqlTail
	sqlCnt += ") as combined"
	if err := d.DB.WithContext(ctx).Raw(sqlCnt).Scan(&count).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed on count user multi chain items")
	}
	if err := d.DB.WithContext(ctx).Raw(sql).Scan(&items).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed on get user multi chain items")
	}

	return items, count, nil
}

func (d *Dao) QueryMultiChainUserListingItemInfos(ctx context.Context, chain []string, userAddrs []string, contractAddrs []string, page, pageSize int) ([]types.PortfolioItemInfo, int64, error) {
	var count int64
	var items []types.PortfolioItemInfo
	var userAddrsParam string
	for i, addr := range userAddrs {
		userAddrsParam += fmt.Sprintf(`'%s'`, addr)
		if i < len(userAddrs)-1 {
			userAddrsParam += ","
		}
	}

	sqlCntHead := "SELECT COUNT(*) FROM ("
	sqlHead := "SELECT * FROM ("
	sqlTail := fmt.Sprintf(") as combined ORDER BY combined.owned_time DESC LIMIT %d OFFSET %d", pageSize, page-1)
	var sqlMids []string

	for _, chainName := range chain {
		//splice sqlMid
		sqlMid := "("
		sqlMid += "select gi.chain_id as chain_id, gi.collection_address as collection_address, gi.token_id as token_id, gi.name as name, gi.owner as owner,0 as rarity_rank, sub.last_event_time as owned_time "
		sqlMid += fmt.Sprintf("from %s gi ", multi.ItemTableName(chainName))
		sqlMid += "left join "
		sqlMid += "(select sgi.collection_address, sgi.token_id, max(sga.event_time) as last_event_time "
		sqlMid += fmt.Sprintf("from %s sgi join %s sga ", multi.ItemTableName(chainName), multi.ActivityTableName(chainName))
		sqlMid += "on sgi.collection_address = sga.collection_address and sgi.token_id = sga.token_id "
		sqlMid += fmt.Sprintf("where sgi.owner in (%s) and sga.activity_type = %d ", userAddrsParam, multi.Sale)
		if len(contractAddrs) > 0 {
			sqlMid += fmt.Sprintf("and sgi.collection_address in ('%s'", contractAddrs[0])
			for i := 1; i < len(contractAddrs); i++ {
				sqlMid += fmt.Sprintf(",'%s'", contractAddrs[i])
			}
			sqlMid += ") "
		}
		sqlMid += "group by sgi.collection_address, sgi.token_id) sub "
		sqlMid += "on gi.collection_address = sub.collection_address and gi.token_id = sub.token_id "
		sqlMid += fmt.Sprintf("where gi.owner in (%s) ", userAddrsParam)
		if len(contractAddrs) > 0 {
			sqlMid += fmt.Sprintf("and gi.collection_address in ('%s'", contractAddrs[0])
			for i := 1; i < len(contractAddrs); i++ {
				sqlMid += fmt.Sprintf(",'%s'", contractAddrs[i])
			}
			sqlMid += ")"
		}
		sqlMid += ")"
		//store into slice
		sqlMids = append(sqlMids, sqlMid)
	}

	sqlCnt := sqlCntHead
	sql := sqlHead
	for i := 0; i < len(sqlMids); i++ {
		if i != 0 {
			sql += " UNION ALL "
			sqlCnt += " UNION ALL "
		}
		sql += sqlMids[i]
		sqlCnt += sqlMids[i]
	}
	sql += sqlTail
	sqlCnt += ") as combined"
	if err := d.DB.WithContext(ctx).Raw(sqlCnt).Scan(&count).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed on count user multi chain items")
	}
	if err := d.DB.WithContext(ctx).Raw(sql).Scan(&items).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed on get user multi chain items")
	}

	return items, count, nil
}

func (d *Dao) QueryCollectionsListed(ctx context.Context, chain string, collectionAddrs []string) ([]types.CollectionListed, error) {
	var collectionsListed []types.CollectionListed
	if len(collectionAddrs) == 0 {
		return collectionsListed, nil
	}

	for _, address := range collectionAddrs {
		count, err := d.KvStore.GetInt(ordermanager.GenCollectionListedKey(chain, address))
		if err != nil {
			return nil, errors.Wrap(err, "failed on set collection listed count")
		}
		collectionsListed = append(collectionsListed, types.CollectionListed{
			CollectionAddr: address,
			Count:          count,
		})
	}

	return collectionsListed, nil
}

func (d *Dao) CacheCollectionsListed(ctx context.Context, chain string, collectionAddr string, listedCount int) error {
	err := d.KvStore.SetInt(ordermanager.GenCollectionListedKey(chain, collectionAddr), listedCount)
	if err != nil {
		return errors.Wrap(err, "failed on set collection listed count")
	}

	return nil
}

func (d *Dao) QueryFloorPrice(ctx context.Context, chain string, collectionAddr string) (decimal.Decimal, error) {
	var order multi.Order
	sql := fmt.Sprintf(`SELECT co.price as price
FROM %s as ci
         left join %s co on co.collection_address = ci.collection_address and co.token_id = ci.token_id
WHERE (co.collection_address= ? and co.order_type = ? and
       co.order_status = ? and co.maker = ci.owner and (ci.is_opensea_banned, co.marketplace_id) != (?, ?))
 order by co.price asc limit 1`, multi.ItemTableName(chain), multi.OrderTableName(chain))
	if err := d.DB.WithContext(ctx).Raw(
		sql,
		collectionAddr,
		OrderType,
		OrderStatus,
		true,
		1,
	).Scan(&order).Error; err != nil {
		return decimal.Zero, errors.Wrap(err, "failed on get collection floor price")
	}

	return order.Price, nil
}

func GetCollectionTradeInfoKey(project, chain string, collectionAddr string) string {
	return fmt.Sprintf("cache:%s:%s:collection:%s:trade", strings.ToLower(project), strings.ToLower(chain), strings.ToLower(collectionAddr))
}

type CollectionVolume struct {
	Volume decimal.Decimal `json:"volume"`
}

func (d *Dao) QueryCollectionAllVolume(project, chain string, collectionAddr string) (*CollectionVolume, error) {
	key := GetCollectionTradeInfoKey(project, chain, collectionAddr)
	rawInfo, err := d.KvStore.Get(key)
	if err != nil {
		return nil, errors.Wrap(err, "failed on get collection all volume")
	}

	var vol CollectionVolume
	if err := json.Unmarshal([]byte(rawInfo), &vol); err != nil {
		return nil, errors.Wrap(err, "failed on unmarshal volume info")
	}

	return &vol, nil
}

func GetHoldersCountKey(chain string) string {
	return fmt.Sprintf("cache:orderbookdex:%s:holders:count", chain)
}

func (d *Dao) QueryCollectionFloorChange(chain string, timeDiff int64) (map[string]float64, error) {
	collectionFloorChange := make(map[string]float64)

	var collectionPrices []multi.CollectionFloorPrice
	rawSql := fmt.Sprintf("SELECT collection_address, price, event_time FROM %s WHERE (collection_address, event_time) IN (    SELECT collection_address, MAX(event_time)    FROM %s    GROUP BY collection_address) OR (collection_address, event_time) IN (    SELECT collection_address, MAX(event_time)    FROM %s WHERE event_time <= UNIX_TIMESTAMP() - ? GROUP BY collection_address) ORDER BY collection_address,event_time DESC", multi.CollectionFloorPriceTableName(chain), multi.CollectionFloorPriceTableName(chain), multi.CollectionFloorPriceTableName(chain))
	if err := d.DB.Raw(rawSql, timeDiff).Scan(&collectionPrices).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get collection floor change")
	}

	for i := 0; i < len(collectionPrices); i++ {
		if i < len(collectionPrices)-1 && collectionPrices[i].CollectionAddress == collectionPrices[i+1].CollectionAddress && collectionPrices[i+1].Price.GreaterThan(decimal.Zero) {
			collectionFloorChange[collectionPrices[i].CollectionAddress] = collectionPrices[i].Price.Sub(collectionPrices[i+1].Price).Div(collectionPrices[i+1].Price).InexactFloat64()
			i++
		} else {
			collectionFloorChange[collectionPrices[i].CollectionAddress] = 0.0
		}
	}

	return collectionFloorChange, nil
}

func (d *Dao) QueryCollectionsSellPrice(ctx context.Context, chain string) ([]multi.Collection, error) {
	var collections []multi.Collection
	sql := fmt.Sprintf(`SELECT collection_address as address, max(co.price) as sale_price
FROM %s as co where order_status = ? and order_type = ? and expire_time > ? group by collection_address`, multi.OrderTableName(chain))
	if err := d.DB.WithContext(ctx).Raw(
		sql,
		multi.OrderStatusActive,
		multi.CollectionBidOrder,
		time.Now().Unix()).Scan(&collections).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get collection sell price")
	}

	return collections, nil
}

func (d *Dao) QueryCollectionSellPrice(ctx context.Context, chain, collectionAddr string) (*multi.Collection, error) {
	var collection multi.Collection
	sql := fmt.Sprintf(`SELECT collection_address as address, co.price as sale_price
FROM %s as co where collection_address = ? and order_status = ? and order_type = ? and quantity_remaining > 0 and expire_time > ? order by price desc limit 1`, multi.OrderTableName(chain))
	if err := d.DB.WithContext(ctx).Raw(
		sql,
		collectionAddr,
		multi.OrderStatusActive,
		multi.CollectionBidOrder,
		time.Now().Unix()).Scan(&collection).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get collection sell price")
	}

	return &collection, nil
}
