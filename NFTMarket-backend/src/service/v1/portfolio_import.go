package service

import (
	"context"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"

	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
	"github.com/ProjectsTask/EasySwapBase/stores/gdb/orderbookmodel/multi"
)

const maxImageURILength = 255

func ImportPortfolioItem(ctx context.Context, svcCtx *svc.ServerCtx, chainName string, req *types.PortfolioImportItemRequest) (*types.PortfolioImportItemResponse, error) {
	nodeSrv, ok := svcCtx.NodeSrvs[int64(req.ChainID)]
	if !ok {
		return nil, errors.New("chain service not found")
	}

	ownerAddr, err := nodeSrv.FetchNftOwner(req.CollectionAddress, req.TokenID)
	if err != nil {
		return nil, errors.Wrap(err, "token does not exist or owner query failed")
	}

	if !strings.EqualFold(ownerAddr.Hex(), req.UserAddress) {
		return nil, errors.New("token owner does not match user address")
	}

	metadata, _ := nodeSrv.FetchOnChainMetadata(req.CollectionAddress, req.TokenID)
	itemName := "NFT #" + req.TokenID
	imageURI := strings.TrimSpace(req.LogoURI)
	if metadata != nil {
		if strings.TrimSpace(metadata.Name) != "" {
			itemName = metadata.Name
		}
		if imageURI == "" && strings.TrimSpace(metadata.Image) != "" {
			imageURI = metadata.Image
		}
	}
	imageURI = sanitizeImageURI(imageURI)

	collectionTable := multi.CollectionTableName(chainName)
	itemTable := multi.ItemTableName(chainName)
	itemExternalTable := multi.ItemExternalTableName(chainName)

	now := time.Now().Unix()
	collection := map[string]interface{}{
		"address":        strings.ToLower(req.CollectionAddress),
		"chain_id":       req.ChainID,
		"symbol":         "UNKNOWN",
		"name":           "Unknown Collection",
		"image_uri":      imageURI,
		"creator":        ownerAddr.Hex(),
		"token_standard": int64(1),
		"auth":           0,
		"owner_amount":   0,
		"item_amount":    0,
		"floor_price":    decimal.Zero,
		"sale_price":     decimal.Zero,
		"volume_total":   decimal.Zero,
		"create_time":    now,
		"update_time":    now,
	}
	if err := svcCtx.Dao.DB.WithContext(ctx).Table(collectionTable).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "address"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"image_uri": imageURI, "update_time": now}),
	}).Create(&collection).Error; err != nil {
		return nil, errors.Wrap(err, "failed to upsert collection")
	}

	item := map[string]interface{}{
		"chain_id":           req.ChainID,
		"collection_address": strings.ToLower(req.CollectionAddress),
		"token_id":           req.TokenID,
		"name":               itemName,
		"owner":              ownerAddr.Hex(),
		"creator":            ownerAddr.Hex(),
		"supply":             1,
		"list_price":         decimal.Zero,
		"list_time":          0,
		"sale_price":         decimal.Zero,
		"create_time":        now,
		"update_time":        now,
	}
	if err := svcCtx.Dao.DB.WithContext(ctx).Table(itemTable).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "collection_address"}, {Name: "token_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"owner": ownerAddr.Hex(), "name": itemName, "update_time": now}),
	}).Create(&item).Error; err != nil {
		return nil, errors.Wrap(err, "failed to upsert item")
	}

	if imageURI != "" {
		itemExternal := map[string]interface{}{
			"collection_address": strings.ToLower(req.CollectionAddress),
			"token_id":           req.TokenID,
			"image_uri":          imageURI,
			"create_time":        now,
			"update_time":        now,
		}
		_ = svcCtx.Dao.DB.WithContext(ctx).Table(itemExternalTable).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "collection_address"}, {Name: "token_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"image_uri":   imageURI,
				"update_time": now,
			}),
		}).Create(&itemExternal).Error
	}

	return &types.PortfolioImportItemResponse{
		ChainID:           req.ChainID,
		CollectionAddress: strings.ToLower(req.CollectionAddress),
		TokenID:           req.TokenID,
		Owner:             ownerAddr.Hex(),
		Name:              itemName,
		ImageURI:          imageURI,
	}, nil
}

func ValidatePortfolioImportRequest(req *types.PortfolioImportItemRequest) error {
	if req.ChainID <= 0 {
		return errors.New("invalid chain_id")
	}
	if !common.IsHexAddress(req.UserAddress) {
		return errors.New("invalid user_address")
	}
	if !common.IsHexAddress(req.CollectionAddress) {
		return errors.New("invalid collection_address")
	}
	if strings.TrimSpace(req.TokenID) == "" {
		return errors.New("token_id is required")
	}
	return nil
}

func sanitizeImageURI(imageURI string) string {
	uri := strings.TrimSpace(imageURI)
	if uri == "" {
		return ""
	}
	// DB 字段长度有限，且 data URI 不适合直接入库。
	if strings.HasPrefix(strings.ToLower(uri), "data:image/") {
		return ""
	}
	if len(uri) > maxImageURILength {
		return ""
	}
	return uri
}
