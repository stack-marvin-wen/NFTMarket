package dao

import (
	"context"
	"fmt"

	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/base"
	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/multi"
	"github.com/pkg/errors"
)

func (d *Dao) GetUserSigStatus(ctx context.Context, userAddr string) (bool, error) {
	var userInfo base.User
	db := d.DB.WithContext(ctx).Table(base.UserTableName()).
		Where("address = ?", userAddr).
		Find(&userInfo)
	if db.Error != nil {
		return false, errors.Wrap(db.Error, "failed on get user info")
	}

	return userInfo.IsSigned, nil
}

func (d *Dao) QueryUserBids(ctx context.Context, chain string, userAddrs []string, contractAddrs []string) ([]multi.Order, error) {
	var userBids []multi.Order
	db := d.DB.WithContext(ctx).Table(fmt.Sprintf("%s", multi.OrderTableName(chain))).
		Select("collection_address, token_id, order_id, token_id,order_type,quantity_remaining, size, event_time, price, salt, expire_time").
		Where("maker in (?) and order_type in (?,?) and order_status = ? and quantity_remaining > 0", userAddrs, multi.ItemBidOrder, multi.CollectionBidOrder, multi.OrderStatusActive)
	if len(contractAddrs) != 0 {
		db.Where("collection_address in (?)", contractAddrs)
	}

	if err := db.Scan(&userBids).Error; err != nil {
		return nil, errors.Wrap(err, "failed on get user bids")
	}

	return userBids, nil
}
