package dao

import (
	"context"
	"fmt"
	"strings"

	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/multi"
	"github.com/pkg/errors"
)

func (d *Dao) QueryCollectionItemsImage(ctx context.Context, chain string, collectionAddr string, tokenIds []string) ([]multi.ItemExternal, error) {
	var itemsExternal []multi.ItemExternal
	if err := d.DB.WithContext(ctx).Table(multi.ItemExternalTableName(chain)).
		Select("collection_address, token_id, is_uploaded_oss, image_uri, oss_uri,video_type,is_video_uploaded,video_uri,video_oss_uri").
		Where("collection_address = ? and token_id in (?)", collectionAddr, tokenIds).
		Scan(&itemsExternal).Error; err != nil {
		return nil, errors.Wrap(err, "failed on query items external info")
	}

	return itemsExternal, nil
}

func (d *Dao) QueryMultiChainCollectionsItemsImage(ctx context.Context, itemInfos []MultiChainItemInfo) ([]multi.ItemExternal, error) {
	var itemsExternal []multi.ItemExternal

	sqlHead := "SELECT * FROM ("
	sqlTail := ") as combined"
	var sqlMids []string

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

	for chainName, items := range chainItems {
		tmpStat := fmt.Sprintf("(('%s','%s')", items[0].CollectionAddress, items[0].TokenID)
		for i := 1; i < len(items); i++ {
			tmpStat += fmt.Sprintf(",('%s','%s')", items[i].CollectionAddress, items[i].TokenID)
		}
		tmpStat += ") "

		sqlMid := "("
		sqlMid += "select collection_address, token_id, is_uploaded_oss, image_uri, oss_uri "
		sqlMid += fmt.Sprintf("from %s ", multi.ItemExternalTableName(chainName))
		sqlMid += "where (collection_address,token_id) in "
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

	if err := d.DB.WithContext(ctx).Raw(sql).Scan(&itemsExternal).Error; err != nil {
		return nil, errors.Wrap(err, "failed on query multi chain items external info")
	}

	return itemsExternal, nil
}
