package dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/multi"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
)

const (
	BuyNow   = 1
	HasOffer = 2
	All      = 3
)

const (
	listTime      = 0
	listPriceAsc  = 1
	listPriceDesc = 2
	salePriceDesc = 3
	salePriceAsc  = 4
)

type CollectionItem struct {
	multi.Item
	MarketID       int    `json:"market_id"`
	Listing        bool   `json:"listing"`
	OrderID        string `json:"order_id"`
	OrderStatus    int    `json:"order_status"`
	ListMaker      string `json:"list_maker"`
	ListTime       int64  `json:"list_time"`
	ListExpireTime int64  `json:"list_expire_time"`
	ListSalt       int64  `json:"list_salt"`
}

func (d *Dao) QueryCollectionBids(ctx context.Context, chain string, collectionAddr string, page, pageSize int) ([]types.CollectionBids, int64, error) {
	var count int64

	if err := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s", multi.OrderTableName(chain))).Where("collection_address = ? and order_type = ? and order_status = ? and expire_time > ?", collectionAddr, multi.CollectionBidOrder, multi.OrderStatusActive, time.Now().Unix()).
		Group("price").Count(&count).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed on count user items")
	}

	var bids []types.CollectionBids
	db := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s", multi.OrderTableName(chain)))
	if err := db.Select("sum(quantity_remaining) AS size, price, sum(quantity_remaining)*price as total, COUNT(DISTINCT maker) AS bidders").
		Where("collection_address = ? and order_type = ? and order_status = ? and expire_time > ? and quantity_remaining > 0", collectionAddr, multi.CollectionBidOrder, multi.OrderStatusActive, time.Now().Unix()).
		Group("price").Order("price desc").Limit(int(pageSize)).Offset(int(pageSize * (page - 1))).
		Scan(&bids).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed on query collection bids")
	}

	return bids, count, nil
}

func (d *Dao) QueryCollectionItemOrder(ctx context.Context, chain string, filter types.CollectionItemFilterParams, collectionAddr string) ([]*CollectionItem, int64, error) {
	if len(filter.Markets) == 0 {
		filter.Markets = []int{int(multi.MarketOrderBook)}
	}

	db := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s as ci", multi.ItemTableName(chain)))
	coTableName := multi.OrderTableName(chain)
	// status 1 buy now  2 has offer  3 all
	if len(filter.Status) == 1 {
		db.Select("ci.id as id, ci.chain_id as chain_id, 0 as rarity_rank," +
			"ci.collection_address as collection_address,ci.token_id as token_id, ci.name as name, ci.owner as owner, " +
			"ci.is_opensea_banned as is_opensea_banned, " +
			"min(co.price) as list_price, SUBSTRING_INDEX(GROUP_CONCAT(co.marketplace_id ORDER BY co.price,co.marketplace_id),',', 1) AS market_id, min(co.price) != 0 as listing")

		// buy now
		if filter.Status[0] == BuyNow {
			db.Joins(fmt.Sprintf("join %s co on co.collection_address=ci.collection_address and co.token_id=ci.token_id", coTableName)).
				Where("co.collection_address = ? and co.order_type = ? and co.order_status=? and lower(co.maker) = lower(ci.owner)"+
					" and (ci.is_opensea_banned,co.marketplace_id)!=(true,1)",
					collectionAddr, multi.ListingOrder, multi.OrderStatusActive)
			if len(filter.Markets) == 1 {
				db.Where("co.marketplace_id = ?", filter.Markets[0])
			} else if len(filter.Markets) != 5 {
				db.Where("co.marketplace_id in (?)", filter.Markets)
			}
			if filter.TokenID != "" {
				db.Where("co.token_id =?", filter.TokenID)
			}
			if filter.UserAddress != "" {
				db.Where("ci.owner =?", filter.UserAddress)
			}

			db.Group("co.token_id")
		}
		//has offer
		if filter.Status[0] == HasOffer {
			db.Joins(fmt.Sprintf("join %s co on co.collection_address=ci.collection_address and co.token_id=ci.token_id", coTableName)).
				Where("co.collection_address = ? and co.order_type = ? and co.order_status = ?",
					collectionAddr, multi.OfferOrder, multi.OrderStatusActive)
			if len(filter.Markets) == 1 {
				db.Where("co.marketplace_id = ?", filter.Markets[0])
			} else if len(filter.Markets) != 5 {
				db.Where("co.marketplace_id in (?)", filter.Markets)
			}
			if filter.TokenID != "" {
				db.Where("co.token_id =?", filter.TokenID)
			}
			if filter.UserAddress != "" {
				db.Where("ci.owner =?", filter.UserAddress)
			}
			db.Group("co.token_id")
		}
	} else if len(filter.Status) == 2 {
		// buy and sell
		db.Select("ci.id as id, ci.chain_id as chain_id, 0 as rarity_rank," +
			"ci.collection_address as collection_address,ci.token_id as token_id, ci.name as name, ci.owner as owner, " +
			"ci.is_opensea_banned as is_opensea_banned, " +
			"min(co.price) as list_price, SUBSTRING_INDEX(GROUP_CONCAT(co.marketplace_id ORDER BY co.price,co.marketplace_id),',', 1) AS market_id")
		// join list item
		db.Joins(fmt.Sprintf("join %s co on co.collection_address=ci.collection_address and co.token_id=ci.token_id", coTableName)).
			Where("co.collection_address = ? and  co.order_status=? and lower(co.maker) = lower(ci.owner)"+
				" and (ci.is_opensea_banned,co.marketplace_id)!=(true,1)",
				collectionAddr, multi.OrderStatusActive)
		if len(filter.Markets) == 1 {
			db.Where("co.marketplace_id = ?", filter.Markets[0])
		} else if len(filter.Markets) != 5 {
			db.Where("co.marketplace_id in (?)", filter.Markets)
		}
		if filter.TokenID != "" {
			db.Where("co.token_id =?", filter.TokenID)
		}
		if filter.UserAddress != "" {
			db.Where("ci.owner =?", filter.UserAddress)
		}

		db.Group("co.token_id").Having("min(co.type)=? and max(co.type)=?", multi.ListingOrder, multi.OfferOrder)
	} else {
		subQuery := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s as cis", multi.ItemTableName(chain))).
			Select("cis.id as item_id,cis.collection_address as collection_address,cis.token_id as token_id, cis.owner as owner, cos.order_id as order_id, min(cos.price) as list_price, SUBSTRING_INDEX(GROUP_CONCAT(cos.marketplace_id ORDER BY cos.price,cos.marketplace_id),',', 1) AS market_id, min(cos.price) != 0 as listing").
			Joins(fmt.Sprintf("join %s cos on cos.collection_address=cis.collection_address and cos.token_id=cis.token_id", coTableName)).
			Where("cos.collection_address = ? and cos.order_type = ? and cos.order_status=? and lower(cos.maker) = lower(cis.owner)"+
				" and (cis.is_opensea_banned,cos.marketplace_id)!=(true,1)",
				collectionAddr, multi.ListingOrder, multi.OrderStatusActive)

		if len(filter.Markets) == 1 {
			subQuery.Where("cos.marketplace_id = ?", filter.Markets[0])
		} else if len(filter.Markets) != 5 {
			subQuery.Where("cos.marketplace_id in (?)", filter.Markets)
		}
		subQuery.Group("cos.token_id")

		db.Joins("left join (?) co on co.collection_address=ci.collection_address and co.token_id=ci.token_id", subQuery).
			Select("ci.id as id, ci.chain_id as chain_id, 0 as rarity_rank," +
				"ci.collection_address as collection_address, ci.token_id as token_id, ci.name as name, ci.owner as owner, " +
				" ci.is_opensea_banned as is_opensea_banned, " +
				"co.list_price as list_price, co.market_id as market_id, co.listing as listing").
			Where(fmt.Sprintf("ci.collection_address = '%s'", collectionAddr))
		if filter.TokenID != "" {
			db.Where(fmt.Sprintf("ci.token_id = '%s'", filter.TokenID))
		}
		if filter.UserAddress != "" {
			db.Where(fmt.Sprintf("ci.owner = '%s'", filter.UserAddress))
		}
	}

	var count int64
	countTx := db.Session(&gorm.Session{})
	if err := countTx.Count(&count).Error; err != nil {
		return nil, 0, errors.Wrap(db.Error, "failed on count items")
	}

	// select all collection items
	if len(filter.Status) == 0 {
		db.Order("listing desc")
	}

	if filter.Sort == 0 {
		filter.Sort = listPriceAsc
	}

	switch filter.Sort {
	case listTime:
		db.Order("list_time desc,ci.id asc")
	case listPriceAsc:
		db.Order("list_price asc, ci.id asc")
	case listPriceDesc:
		db.Order("list_price desc,ci.id asc")
	case salePriceDesc:
		db.Order("sale_price desc,ci.id asc")
	case salePriceAsc:
		db.Order("sale_price = 0,sale_price asc,ci.id asc")
	}

	var items []*CollectionItem
	db.Offset(int((filter.Page - 1) * filter.PageSize)).Limit(int(filter.PageSize)).Scan(&items)
	if db.Error != nil {
		return nil, 0, errors.Wrap(db.Error, "failed on get query items info")
	}

	return items, count, nil
}

// QueryCollectionItemBySupply 仅基于 ob_item 表查询可售 NFT（不关联订单表）。
// 业务约定：
// - supply >= 1 视为可售
// - 对外返回的 list_price 使用 item.sale_price
func (d *Dao) QueryCollectionItemBySupply(ctx context.Context, chain string, filter types.CollectionItemFilterParams, collectionAddr string) ([]*CollectionItem, int64, error) {
	db := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s as ci", multi.ItemTableName(chain))).
		Select("ci.id as id, ci.chain_id as chain_id, 0 as rarity_rank,"+
			"ci.collection_address as collection_address, ci.token_id as token_id, ci.name as name, ci.owner as owner,"+
			"ci.is_opensea_banned as is_opensea_banned, "+
			"ci.sale_price as list_price, 0 as market_id, (ci.supply >= 1 and ci.sale_price is not null and ci.sale_price > 0) as listing").
		Where("ci.collection_address = ? AND ci.supply >= 1", collectionAddr)

	if filter.TokenID != "" {
		db.Where("ci.token_id = ?", filter.TokenID)
	}
	if filter.UserAddress != "" {
		db.Where("ci.owner = ?", filter.UserAddress)
	}

	var count int64
	countTx := db.Session(&gorm.Session{})
	if err := countTx.Count(&count).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed on count items")
	}

	if filter.Sort == 0 {
		filter.Sort = listPriceAsc
	}

	switch filter.Sort {
	case listTime:
		db.Order("ci.list_time desc, ci.id asc")
	case listPriceAsc:
		db.Order("list_price asc, ci.id asc")
	case listPriceDesc:
		db.Order("list_price desc, ci.id asc")
	case salePriceDesc:
		db.Order("list_price desc, ci.id asc")
	case salePriceAsc:
		db.Order("list_price = 0, list_price asc, ci.id asc")
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	var items []*CollectionItem
	if err := db.Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Scan(&items).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed on get query items info")
	}

	return items, count, nil
}

type UserItemCount struct {
	Owner  string `json:"owner"`
	Counts int64  `json:"counts"`
}

func (d *Dao) QueryUsersItemCount(ctx context.Context, chain string, collectionAddr string, owners []string) ([]UserItemCount, error) {
	var itemCount []UserItemCount
	if err := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s as ci", multi.ItemTableName(chain))).
		Select("owner,COUNT(*) AS counts").Where("collection_address = ? and owner in (?)", collectionAddr, owners).
		Group("owner").Scan(&itemCount).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get user item count")
	}
	return itemCount, nil
}

func (d *Dao) QueryLastSalePrice(ctx context.Context, chain string, collectionAddr string, tokenIds []string) ([]multi.Activity, error) {
	var lastSales []multi.Activity
	sql := fmt.Sprintf(`
SELECT a.collection_address, a.token_id, a.price
FROM %s a
INNER JOIN (
    SELECT collection_address,token_id, MAX(event_time) as max_event_time
    FROM %s
    WHERE collection_address = ?
      AND token_id IN (?)
      AND activity_type = ?
    GROUP BY collection_address,token_id
) groupedA 
ON a.collection_address = groupedA.collection_address
AND a.token_id = groupedA.token_id
AND a.event_time = groupedA.max_event_time
AND a.activity_type = ?
`, multi.ActivityTableName(chain), multi.ActivityTableName(chain))
	if err := d.DB.Raw(sql, collectionAddr, tokenIds, multi.Sale, multi.Sale).Scan(&lastSales).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get item last sale price")
	}

	return lastSales, nil
}

func (d *Dao) QueryBestBids(ctx context.Context, chain string, userAddr string, collectionAddr string, tokenIds []string) ([]multi.Order, error) {
	var bestBids []multi.Order
	var sql string
	if userAddr == "" {
		sql = fmt.Sprintf(`
SELECT order_id, token_id, event_time, price, salt, expire_time,maker,order_type,quantity_remaining,size   
    FROM %s
    WHERE collection_address = ?
      AND token_id IN (?)
      AND order_type = ?
      AND order_status = ?
      AND expire_time > ?
	  AND quantity_remaining > 0
`, multi.OrderTableName(chain))
	} else {
		sql = fmt.Sprintf(`
SELECT order_id, token_id, event_time, price, salt, expire_time,maker,order_type,quantity_remaining,size   
    FROM %s
    WHERE collection_address = ?
      AND token_id IN (?)
      AND order_type = ?
      AND order_status = ?
      AND expire_time > ?
	  AND quantity_remaining > 0
      AND maker != '%s'
`, multi.OrderTableName(chain), userAddr)
	}

	if err := d.DB.Raw(sql, collectionAddr, tokenIds, multi.ItemBidOrder, multi.OrderStatusActive, time.Now().Unix()).Scan(&bestBids).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get item best bids")
	}

	return bestBids, nil
}

func (d *Dao) QueryItemsBestBids(ctx context.Context, chain string, userAddr string, itemInfos []types.ItemInfo) ([]multi.Order, error) {
	var conditions []clause.Expr
	for _, info := range itemInfos {
		conditions = append(conditions, gorm.Expr("(?, ?)", info.CollectionAddress, info.TokenID))
	}

	var bestBids []multi.Order
	var sql string
	if userAddr == "" {
		sql = fmt.Sprintf(`
SELECT order_id, token_id, event_time, price, salt, expire_time, maker, order_type, quantity_remaining, size
    FROM %s
    WHERE (collection_address,token_id) IN (?)
      AND order_type = ?
      AND order_status = ?
	  AND quantity_remaining > 0
      AND expire_time > ?
`, multi.OrderTableName(chain))
	} else {
		sql = fmt.Sprintf(`
SELECT order_id, token_id, event_time, price, salt, expire_time, maker, order_type, quantity_remaining,size 
    FROM %s
    WHERE (collection_address,token_id) IN (?)
      AND order_type = ?
      AND order_status = ?
	  AND quantity_remaining > 0
      AND expire_time > ?
	  AND maker != '%s'
`, multi.OrderTableName(chain), userAddr)
	}

	if err := d.DB.Raw(sql, conditions, multi.ItemBidOrder, multi.OrderStatusActive, time.Now().Unix()).Scan(&bestBids).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get item best bids")
	}

	return bestBids, nil
}

func (d *Dao) QueryCollectionsBestBid(ctx context.Context, chain string, userAddr string, collectionAddrs []string) ([]*multi.Order, error) {
	var bestBid []*multi.Order
	sql := fmt.Sprintf(`
	SELECT collection_address, order_id, price,event_time, expire_time, salt, maker, order_type, quantity_remaining, size  
    FROM %s `, multi.OrderTableName(chain))
	sql += fmt.Sprintf(`where (collection_address,price) in (SELECT collection_address, max(price) as price FROM %s `, multi.OrderTableName(chain))
	sql += fmt.Sprintf(`where collection_address in (?) and order_type = ? and order_status = ? and quantity_remaining > 0 and expire_time > ? `)
	if userAddr != "" {
		sql += fmt.Sprintf(" and maker != '%s'", userAddr)
	}
	sql += fmt.Sprintf(`group by collection_address ) `)
	sql += fmt.Sprintf(`and order_type = ? and order_status = ? and quantity_remaining > 0 and expire_time > ? `)
	if userAddr != "" {
		sql += fmt.Sprintf(" and maker != '%s'", userAddr)
	}

	if err := d.DB.Raw(sql, collectionAddrs, multi.CollectionBidOrder, multi.OrderStatusActive, time.Now().Unix(), multi.CollectionBidOrder, multi.OrderStatusActive, time.Now().Unix()).Scan(&bestBid).Error; err != nil {
		return bestBid, errors.Wrap(err, "failed on get item best bids")
	}

	return bestBid, nil
}

func (d *Dao) QueryCollectionBestBid(ctx context.Context, chain string, userAddr string, collectionAddr string) (multi.Order, error) {
	var bestBid multi.Order
	var sql string
	if userAddr == "" {
		sql = fmt.Sprintf(`
	SELECT order_id, price,event_time, expire_time, salt, maker, order_type, quantity_remaining, size  
    FROM %s
    WHERE collection_address = ?
      AND order_type = ?
      AND order_status = ?
	  AND quantity_remaining > 0
      AND expire_time > ? order by price desc limit 1
`, multi.OrderTableName(chain))
	} else {
		sql = fmt.Sprintf(`
	SELECT order_id, price,event_time, expire_time, salt, maker, order_type, quantity_remaining, size  
    FROM %s
    WHERE collection_address = ?
      AND order_type = ?
      AND order_status = ?
	  AND quantity_remaining > 0
      AND expire_time > ? and maker != '%s' order by price desc limit 1
`, multi.OrderTableName(chain), userAddr)
	}
	if err := d.DB.Raw(sql, collectionAddr, multi.CollectionBidOrder, multi.OrderStatusActive, time.Now().Unix()).Scan(&bestBid).Error; err != nil {
		return bestBid, errors.Wrap(err, "failed on get item best bids")
	}

	return bestBid, nil
}

func (d *Dao) QueryCollectionTopNBid(ctx context.Context, chain string, userAddr string, collectionAddr string, num int) ([]multi.Order, error) {
	var bestBids []multi.Order
	var sql string
	if userAddr == "" {
		sql = fmt.Sprintf(`
	SELECT order_id, price,event_time, expire_time, salt, maker, order_type, quantity_remaining, size 
    FROM %s
    WHERE collection_address = ?
      AND order_type = ?
      AND order_status = ?
	  AND quantity_remaining > 0
      AND expire_time > ? order by price desc limit %d
`, multi.OrderTableName(chain), num)
	} else {
		sql = fmt.Sprintf(`
	SELECT order_id, price,event_time, expire_time, salt, maker, order_type, quantity_remaining, size
    FROM %s
    WHERE collection_address = ?
      AND order_type = ?
      AND order_status = ?
	  AND quantity_remaining > 0
      AND expire_time > ? and maker != '%s' order by price desc limit %d
`, multi.OrderTableName(chain), userAddr, num)
	}
	if err := d.DB.Raw(sql, collectionAddr, multi.CollectionBidOrder, multi.OrderStatusActive, time.Now().Unix()).Scan(&bestBids).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get item best bids")
	}

	var results []multi.Order
	for i := 0; i < len(bestBids); i++ {
		for j := 0; j < int(bestBids[i].QuantityRemaining); j++ {
			results = append(results, bestBids[i])
		}
	}
	if num > len(results) {
		return results[:], nil
	}

	return results[:num], nil
}

var collectionDetailFields = []string{"id", "chain_id", "token_standard", "name", "address", "image_uri", "banner_uri", "floor_price", "sale_price", "item_amount", "owner_amount"}

const OrderType = 1
const OrderStatus = 0

func (d *Dao) QueryListedAmount(ctx context.Context, chain string, collectionAddr string) (int64, error) {
	sql := fmt.Sprintf(`SELECT count(distinct (co.token_id)) as counts
FROM %s as ci
         join %s co on co.collection_address = ci.collection_address and co.token_id = ci.token_id
WHERE (co.collection_address=? and co.order_type = ? and
       co.order_status = ? and lower(co.maker) = lower(ci.owner) and (ci.is_opensea_banned, co.marketplace_id) != (?, ?))
`, multi.ItemTableName(chain), multi.OrderTableName(chain))
	var counts int64
	if err := d.DB.WithContext(ctx).Raw(
		sql,
		collectionAddr,
		OrderType,
		OrderStatus,
		true,
		1,
	).Scan(&counts).Error; err != nil {
		return 0, errors.Wrap(err, "failed on get listed item amount")
	}

	return counts, nil
}

func (d *Dao) QueryListedAmountEachCollection(ctx context.Context, chain string, collectionAddrs []string, userAddrs []string) ([]types.CollectionInfo, error) {
	var counts []types.CollectionInfo

	sql := fmt.Sprintf(`SELECT  ci.collection_address as address, count(distinct (co.token_id)) as list_amount
FROM %s as ci
         join %s co on co.collection_address = ci.collection_address and co.token_id = ci.token_id
WHERE (co.collection_address in (?) and ci.owner in (?) and co.order_type = ? and
       co.order_status = ? and lower(co.maker) = lower(ci.owner) and (ci.is_opensea_banned, co.marketplace_id) != (?, ?)) group by ci.collection_address`,
		multi.ItemTableName(chain), multi.OrderTableName(chain))
	if err := d.DB.WithContext(ctx).Raw(
		sql,
		collectionAddrs,
		userAddrs,
		OrderType,
		OrderStatus,
		true,
		1,
	).Scan(&counts).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get listed item amount")
	}

	return counts, nil
}

type MultiChainItemInfo struct {
	types.ItemInfo
	ChainName string
}

func (d *Dao) QueryMultiChainUserItemsListInfo(ctx context.Context, userAddrs []string, itemInfos []MultiChainItemInfo) ([]*CollectionItem, error) {
	var collectionItems []*CollectionItem
	var userAddrsParam string
	for i, addr := range userAddrs {
		userAddrsParam += fmt.Sprintf(`'%s'`, addr)
		if i < len(userAddrs)-1 {
			userAddrsParam += ","
		}
	}

	chainItems := make(map[string][]MultiChainItemInfo)
	for _, itemInfo := range itemInfos {
		items, ok := chainItems[strings.ToLower(itemInfo.ChainName)]
		if ok {
			items = append(items, itemInfo)
			chainItems[strings.ToLower(itemInfo.ChainName)] = items
		} else {
			chainItems[strings.ToLower(itemInfo.ChainName)] = []MultiChainItemInfo{itemInfo}
		}
	}

	sqlHead := "SELECT * FROM ("
	sqlTail := ") as combined"
	var sqlMids []string

	for chainName, items := range chainItems {
		tmpStat := fmt.Sprintf("(('%s','%s')", items[0].CollectionAddress, items[0].TokenID)
		for i := 1; i < len(items); i++ {
			tmpStat += fmt.Sprintf(",('%s','%s')", items[i].CollectionAddress, items[i].TokenID)
		}
		tmpStat += ") "

		sqlMid := "("
		sqlMid += "select ci.id as id, ci.chain_id as chain_id, 0 as rarity_rank,"
		sqlMid += "ci.collection_address as collection_address,ci.token_id as token_id, ci.name as name, ci.owner as owner,"
		sqlMid += "ci.is_opensea_banned as is_opensea_banned,"
		sqlMid += "min(co.price) as list_price, SUBSTRING_INDEX(GROUP_CONCAT(co.marketplace_id ORDER BY co.price,co.marketplace_id),',', 1) AS market_id, min(co.price) != 0 as listing "
		sqlMid += fmt.Sprintf("from %s as ci ", multi.ItemTableName(chainName))
		sqlMid += fmt.Sprintf("join %s co ", multi.OrderTableName(chainName))
		sqlMid += "on co.collection_address=ci.collection_address and co.token_id=ci.token_id "
		sqlMid += "where (co.collection_address,co.token_id) in "
		sqlMid += tmpStat
		sqlMid += fmt.Sprintf("and co.order_type = %d and co.order_status=%d and lower(co.maker) = lower(ci.owner) and co.maker in (%s) ", multi.ListingOrder, multi.OrderStatusActive, userAddrsParam)
		sqlMid += "group by co.collection_address,co.token_id"
		sqlMid += ")"

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

	if err := d.DB.WithContext(ctx).Raw(sql).Scan(&collectionItems).Error; err != nil {
		return nil, errors.Wrap(err, "failed on query user multi chain items list info")
	}

	return collectionItems, nil
}

func (d *Dao) QueryMultiChainUserItemsExpireListInfo(ctx context.Context, userAddrs []string, itemInfos []MultiChainItemInfo) ([]*CollectionItem, error) {
	var collectionItems []*CollectionItem
	var userAddrsParam string
	for i, addr := range userAddrs {
		userAddrsParam += fmt.Sprintf(`'%s'`, addr)
		if i < len(userAddrs)-1 {
			userAddrsParam += ","
		}
	}

	sqlHead := "SELECT * FROM ("
	sqlTail := ") as combined"
	var sqlMids []string

	tmpStat := fmt.Sprintf("(('%s','%s')", itemInfos[0].CollectionAddress, itemInfos[0].TokenID)
	for i := 1; i < len(itemInfos); i++ {
		tmpStat += fmt.Sprintf(",('%s','%s')", itemInfos[i].CollectionAddress, itemInfos[i].TokenID)
	}
	tmpStat += ") "

	for _, info := range itemInfos {
		sqlMid := "("
		sqlMid += "select ci.id as id, ci.chain_id as chain_id, 0 as rarity_rank,"
		sqlMid += "ci.collection_address as collection_address,ci.token_id as token_id, ci.name as name, ci.owner as owner,"
		sqlMid += "ci.is_opensea_banned as is_opensea_banned,co.order_status as order_status,"
		sqlMid += "min(co.price) as list_price, SUBSTRING_INDEX(GROUP_CONCAT(co.marketplace_id ORDER BY co.price,co.marketplace_id),',', 1) AS market_id, min(co.price) != 0 as listing "
		sqlMid += fmt.Sprintf("from %s as ci ", multi.ItemTableName(info.ChainName))
		sqlMid += fmt.Sprintf("join %s co ", multi.OrderTableName(info.ChainName))
		sqlMid += "on co.collection_address=ci.collection_address and co.token_id=ci.token_id "
		sqlMid += "where (co.collection_address,co.token_id) in "
		sqlMid += tmpStat
		sqlMid += fmt.Sprintf("and co.order_type = %d and (co.order_status=%d or co.order_status=%d) and lower(co.maker) = lower(ci.owner) and co.maker in (%s) and (ci.is_opensea_banned,co.marketplace_id) != (true, 1) ", multi.ListingOrder, multi.OrderStatusActive, multi.OrderStatusExpired, userAddrsParam)
		sqlMid += "group by co.collection_address,co.token_id"
		sqlMid += ")"

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

	if err := d.DB.WithContext(ctx).Raw(sql).Scan(&collectionItems).Error; err != nil {
		return nil, errors.Wrap(err, "failed on query user multi chain items list info")
	}

	return collectionItems, nil
}

func (d *Dao) QueryItemListInfo(ctx context.Context, chain, collectionAddr, tokenID string) (*CollectionItem, error) {
	var collectionItem CollectionItem
	db := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s as ci", multi.ItemTableName(chain)))
	coTableName := multi.OrderTableName(chain)

	err := db.Select("ci.id as id, ci.chain_id as chain_id, 0 as rarity_rank,"+
		"ci.collection_address as collection_address,ci.token_id as token_id, ci.name as name, ci.owner as owner, "+
		"ci.is_opensea_banned as is_opensea_banned, "+
		"min(co.price) as list_price, SUBSTRING_INDEX(GROUP_CONCAT(co.marketplace_id ORDER BY co.price,co.marketplace_id),',', 1) AS market_id, min(co.price) != 0 as listing").
		Joins(fmt.Sprintf("join %s co on co.collection_address=ci.collection_address and co.token_id=ci.token_id", coTableName)).
		Where("ci.collection_address =? and ci.token_id = ? and co.order_type = ? and co.order_status=? and lower(co.maker) = lower(ci.owner) "+
			"and (ci.is_opensea_banned,co.marketplace_id) != (true, 1)",
			collectionAddr, tokenID, multi.ListingOrder, multi.OrderStatusActive).
		Group("ci.collection_address,ci.token_id").
		Scan(&collectionItem).Error

	if err != nil {
		return nil, errors.Wrap(err, "failed on query user items list info")
	}

	if !collectionItem.Listing {
		return &collectionItem, nil
	}

	var listOrder multi.Order
	if err := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s as ci", multi.OrderTableName(chain))).
		Select("order_id, expire_time, maker, salt, event_time").Where("collection_address=? and token_id=? and maker=? and order_status=? and price = ?",
		collectionItem.CollectionAddress, collectionItem.TokenId, collectionItem.Owner, multi.OrderStatusActive, collectionItem.ListPrice).
		Scan(&listOrder).Error; err != nil {
		return nil, errors.Wrap(err, "failed on query item order id")
	}
	collectionItem.OrderID = listOrder.OrderID
	collectionItem.ListExpireTime = listOrder.ExpireTime
	collectionItem.ListMaker = listOrder.Maker
	collectionItem.ListSalt = listOrder.Salt
	collectionItem.ListTime = listOrder.EventTime

	return &collectionItem, nil
}

func (d *Dao) QueryListingInfo(ctx context.Context, chain string, priceInfos []types.ItemPriceInfo) ([]multi.Order, error) {
	var conditions []clause.Expr
	for _, price := range priceInfos {
		conditions = append(conditions, gorm.Expr("(?, ?, ?, ?, ?)", price.CollectionAddress, price.TokenID, price.Maker, price.OrderStatus, price.Price))
	}

	var orders []multi.Order
	if err := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s", multi.OrderTableName(chain))).
		Select("collection_address,token_id,order_id,event_time,expire_time,salt,maker,price").Where("(collection_address,token_id,maker,order_status,price) in (?)", conditions).
		Scan(&orders).Error; err != nil {
		return nil, errors.Wrap(err, "failed on query items order id")
	}

	return orders, nil
}

type MultiChainItemPriceInfo struct {
	types.ItemPriceInfo
	ChainName string
}

func (d *Dao) QueryMultiChainListingInfo(ctx context.Context, priceInfos []MultiChainItemPriceInfo) ([]multi.Order, error) {
	var orders []multi.Order
	chainItemPrices := make(map[string][]MultiChainItemPriceInfo)
	for _, priceInfo := range priceInfos {
		items, ok := chainItemPrices[strings.ToLower(priceInfo.ChainName)]
		if ok {
			items = append(items, priceInfo)
			chainItemPrices[strings.ToLower(priceInfo.ChainName)] = items
		} else {
			chainItemPrices[strings.ToLower(priceInfo.ChainName)] = []MultiChainItemPriceInfo{priceInfo}
		}
	}

	sqlHead := "SELECT * FROM ("
	sqlTail := ") as combined"
	var sqlMids []string

	for chainName, priceInfos := range chainItemPrices {
		tmpStat := fmt.Sprintf("(('%s','%s','%s',%d, %s)", priceInfos[0].CollectionAddress, priceInfos[0].TokenID, priceInfos[0].Maker, priceInfos[0].OrderStatus, priceInfos[0].Price.String())
		for i := 1; i < len(priceInfos); i++ {
			tmpStat += fmt.Sprintf(",('%s','%s','%s',%d, %s)", priceInfos[i].CollectionAddress, priceInfos[i].TokenID, priceInfos[i].Maker, priceInfos[i].OrderStatus, priceInfos[i].Price.String())
		}
		tmpStat += ") "

		sqlMid := "("
		sqlMid += "select collection_address,token_id,order_id,salt,event_time,expire_time,maker "
		sqlMid += fmt.Sprintf("from %s ", multi.OrderTableName(chainName))
		sqlMid += "where (collection_address,token_id,maker,order_status,price) in "
		sqlMid += tmpStat
		sqlMid += ")"

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

	if err := d.DB.WithContext(ctx).Raw(sql).Scan(&orders).Error; err != nil {
		return nil, errors.Wrap(err, "failed on query user multi chain order list info")
	}

	return orders, nil
}

func (d *Dao) QueryItemListingAcrossPlatforms(ctx context.Context, chain, collectionAddr, tokenID string, user []string) ([]types.ListingInfo, error) {
	var listings []types.ListingInfo
	if err := d.DB.WithContext(ctx).Table(multi.OrderTableName(chain)).
		Select("marketplace_id, min(price) as price").
		Where("collection_address=? and token_id=? and maker in (?) and order_type=? and order_status = ?",
			collectionAddr,
			tokenID,
			user,
			multi.ListingOrder,
			multi.OrderStatusActive).Group("marketplace_id").Scan(&listings).Error; err != nil {
		return nil, errors.Wrap(err, "failed on query listing from db")
	}

	return listings, nil
}

func (d *Dao) QueryItemInfo(ctx context.Context, chain, collectionAddr, tokenID string) (*multi.Item, error) {
	var item multi.Item
	err := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s as ci", multi.ItemTableName(chain))).Select("ci.id as id, ci.chain_id as chain_id, 0 as rarity_rank,"+
		"ci.collection_address as collection_address,ci.token_id as token_id, ci.name as name, ci.owner as owner, "+
		"ci.is_opensea_banned as is_opensea_banned").
		Where("ci.collection_address =? and ci.token_id = ? ",
			collectionAddr, tokenID).
		Scan(&item).Error

	if err != nil {
		return nil, errors.Wrap(err, "failed on query user items list info")
	}

	return &item, nil
}

func (d *Dao) QueryTraitsPrice(ctx context.Context, chain, collectionAddr string, tokenIds []string) ([]types.TraitPrice, error) {
	var traitsPrice []types.TraitPrice
	listSubQuery := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s as gf_order", multi.OrderTableName(chain))).
		Select("gf_attribute.trait,gf_attribute.trait_value,min(gf_order.price) as price").
		Where("gf_order.collection_address=? and gf_order.order_type=? and gf_order.order_status = ?",
			collectionAddr,
			multi.ListingOrder,
			multi.OrderStatusActive).
		Where("(gf_attribute.trait,gf_attribute.trait_value) in (?)", d.DB.WithContext(ctx).Table(fmt.Sprintf("%s as gf_attr", multi.ItemTraitTableName(chain))).
			Select("gf_attr.trait, gf_attr.trait_value").Where("gf_attr.collection_address=? and gf_attr.token_id in (?)", collectionAddr, tokenIds))
	if err := listSubQuery.Joins(fmt.Sprintf("join %s as gf_attribute on gf_order.collection_address = gf_attribute.collection_address and gf_order.token_id=gf_attribute.token_id", multi.ItemTraitTableName(chain))).
		Group("gf_attribute.trait, gf_attribute.trait_value").Scan(&traitsPrice).Error; err != nil {
		return nil, errors.Wrap(err, "failed on query trait price")
	}
	return traitsPrice, nil
}

func (d *Dao) UpdateItemOwner(ctx context.Context, chain string, collectionAddr, tokenID string, owner string) error {
	if err := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s as ci", multi.ItemTableName(chain))).
		Where("collection_address = ? and token_id = ?", collectionAddr, tokenID).Update("owner", owner).
		Error; err != nil {
		return errors.Wrap(err, "failed on get user item count")
	}
	return nil
}

func (d *Dao) QueryItemBids(ctx context.Context, chain string, collectionAddr, tokenID string, page, pageSize int) ([]types.ItemBid, int64, error) {
	db := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s", multi.OrderTableName(chain))).
		Select("marketplace_id, collection_address, token_id, order_id, salt, event_time, expire_time, price, maker as bidder, order_type, quantity_remaining as bid_unfilled, size as bid_size").
		Where("collection_address = ? and order_type = ? and order_status = ? and expire_time > ? and quantity_remaining > 0", collectionAddr, multi.CollectionBidOrder, multi.OrderStatusActive, time.Now().Unix()).
		Or("collection_address = ? and token_id=? and order_type = ? and order_status = ? and expire_time > ? and quantity_remaining > 0", collectionAddr, tokenID, multi.ItemBidOrder, multi.OrderStatusActive, time.Now().Unix())

	var count int64
	countTx := db.Session(&gorm.Session{})
	if err := countTx.Count(&count).Error; err != nil {
		return nil, 0, errors.Wrap(db.Error, "failed on count user items")
	}

	var itemBids []types.ItemBid
	if count == 0 {
		return itemBids, count, nil
	}
	if err := db.Order("price desc").
		Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Scan(&itemBids).Error; err != nil {
		return nil, 0, errors.Wrap(err, "failed on get user items")
	}

	return itemBids, count, nil
}
