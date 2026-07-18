package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/ProjectsTask/EasySwapBackend/src/service/svc"
	"github.com/ProjectsTask/EasySwapBackend/src/types/v1"
	"github.com/ProjectsTask/EasySwapBase/logger/xzap"
)

// GetCOSTemporaryToken 获取腾讯云COS临时访问凭证
func GetCOSTemporaryToken(ctx context.Context, svcCtx *svc.ServerCtx, userAddr string, req *types.COSTokenRequest) (*types.COSTokenResponse, error) {
	// 验证文件类型
	if !isValidFileType(req.FileType, req.FileName) {
		return nil, errors.New("invalid file type")
	}

	// 验证文件大小
	if !isValidFileSize(req.FileType, req.FileSize) {
		return nil, errors.New("file size exceeds limit")
	}

	// 生成唯一的文件路径
	fileKey := generateFileKey(userAddr, req.FileType, req.FileName)

	// 获取COS配置
	cosConfig := getCOSConfig(svcCtx)

	// 生成临时凭证
	credentials, err := generateTemporaryCredentials(cosConfig, fileKey)
	if err != nil {
		xzap.WithContext(ctx).Error("failed to generate temporary credentials", zap.Error(err))
		return nil, errors.Wrap(err, "failed to generate temporary credentials")
	}

	// 构建响应
	response := &types.COSTokenResponse{
		Credentials:  credentials,
		RequestID:    generateRequestID(),
		StartTime:    time.Now().Format(time.RFC3339),
		Expiration:   time.Now().Add(time.Hour).Format(time.RFC3339),
		ExpiredTime:  time.Now().Add(time.Hour).Unix(),
		Bucket:       cosConfig.Bucket,
		Region:       cosConfig.Region,
		AllowPrefix:  fileKey,
		AllowActions: []string{"name/cos:PutObject", "name/cos:PostObject", "name/cos:PutObjectAcl"},
		UploadURL:    fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cosConfig.Bucket, cosConfig.Region),
		Key:          fileKey,
	}

	return response, nil
}

// COSConfig 腾讯云COS配置
type COSConfig struct {
	SecretID  string
	SecretKey string
	Bucket    string
	Region    string
	AppID     string
}

// getCOSConfig 获取COS配置（从配置文件）
func getCOSConfig(svcCtx *svc.ServerCtx) *COSConfig {
	if svcCtx.C.COS == nil {
		// 如果配置文件中没有COS配置，返回默认配置
		return &COSConfig{
			SecretID:  "YOUR_SECRET_ID",
			SecretKey: "YOUR_SECRET_KEY",
			Bucket:    "nft-assets",
			Region:    "ap-beijing",
			AppID:     "1234567890",
		}
	}

	return &COSConfig{
		SecretID:  svcCtx.C.COS.SecretID,
		SecretKey: svcCtx.C.COS.SecretKey,
		Bucket:    svcCtx.C.COS.Bucket,
		Region:    svcCtx.C.COS.Region,
		AppID:     svcCtx.C.COS.AppID,
	}
}

// generateTemporaryCredentials 生成临时访问凭证
func generateTemporaryCredentials(config *COSConfig, allowPrefix string) (*types.COSCredentials, error) {
	// 当前环境先使用配置中的主密钥直传签名（不下发 session token）。
	// 生产环境建议替换为腾讯云 STS 临时密钥。
	tmpSecretID := strings.TrimSpace(config.SecretID)
	tmpSecretKey := strings.TrimSpace(config.SecretKey)
	if tmpSecretID == "" || tmpSecretKey == "" || strings.Contains(tmpSecretID, "YOUR_") {
		return nil, errors.New("invalid cos secret_id/secret_key config")
	}

	return &types.COSCredentials{
		TmpSecretID:  tmpSecretID,
		TmpSecretKey: tmpSecretKey,
		SessionToken: "",
	}, nil
}

// generateTempSecretID 生成临时SecretID（模拟）
func generateTempSecretID() string {
	randomBytes := make([]byte, 20)
	rand.Read(randomBytes)
	return "AKID" + hex.EncodeToString(randomBytes)
}

// generateTempSecretKey 生成临时SecretKey（模拟）
func generateTempSecretKey() string {
	randomBytes := make([]byte, 20)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

// generateSessionToken 生成会话令牌（模拟）
func generateSessionToken() string {
	randomBytes := make([]byte, 32)
	rand.Read(randomBytes)
	return base64.StdEncoding.EncodeToString(randomBytes)
}

// isValidFileType 验证文件类型
func isValidFileType(fileType, fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))

	validTypes := map[string][]string{
		"image":    {".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"},
		"video":    {".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm"},
		"audio":    {".mp3", ".wav", ".flac", ".aac", ".ogg"},
		"document": {".pdf", ".doc", ".docx", ".txt", ".json"},
	}

	allowedExts, exists := validTypes[fileType]
	if !exists {
		return false
	}

	for _, allowedExt := range allowedExts {
		if ext == allowedExt {
			return true
		}
	}

	return false
}

// isValidFileSize 验证文件大小
func isValidFileSize(fileType string, fileSize int64) bool {
	// 文件大小限制（字节）
	limits := map[string]int64{
		"image":    50 * 1024 * 1024,  // 50MB
		"video":    500 * 1024 * 1024, // 500MB
		"audio":    100 * 1024 * 1024, // 100MB
		"document": 20 * 1024 * 1024,  // 20MB
	}

	limit, exists := limits[fileType]
	if !exists {
		return false
	}

	return fileSize <= limit
}

// generateFileKey 生成文件存储路径
func generateFileKey(userAddr, fileType, fileName string) string {
	// 生成随机字符串
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomStr := hex.EncodeToString(randomBytes)

	// 获取文件扩展名
	ext := filepath.Ext(fileName)

	// 构建文件路径：fileType/userAddr/timestamp_random.ext
	timestamp := time.Now().Unix()
	key := fmt.Sprintf("%s/%s/%d_%s%s",
		fileType,
		userAddr,
		timestamp,
		randomStr,
		ext)

	return key
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

// GetCOSUploadPolicy 获取COS上传策略（备用方法）
func GetCOSUploadPolicy(ctx context.Context, svcCtx *svc.ServerCtx, userAddr string, fileType string) (map[string]interface{}, error) {
	config := getCOSConfig(svcCtx)

	// 生成策略
	policy := map[string]interface{}{
		"expiration": time.Now().Add(time.Hour).Format("2006-01-02T15:04:05.000Z"),
		"conditions": []interface{}{
			map[string]string{"bucket": config.Bucket},
			[]interface{}{"starts-with", "$key", fmt.Sprintf("%s/%s/", fileType, userAddr)},
			[]interface{}{"content-length-range", 0, getMaxFileSize(fileType)},
		},
	}

	return policy, nil
}

// getMaxFileSize 获取文件类型的最大大小
func getMaxFileSize(fileType string) int64 {
	limits := map[string]int64{
		"image":    50 * 1024 * 1024,  // 50MB
		"video":    500 * 1024 * 1024, // 500MB
		"audio":    100 * 1024 * 1024, // 100MB
		"document": 20 * 1024 * 1024,  // 20MB
	}

	if limit, exists := limits[fileType]; exists {
		return limit
	}
	return 10 * 1024 * 1024 // 默认10MB
}
