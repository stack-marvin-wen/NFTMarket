package types

// COSTokenRequest 腾讯云COS临时token请求参数
type COSTokenRequest struct {
	FileType string `json:"file_type" binding:"required"` // 文件类型：image, video, audio, document
	FileName string `json:"file_name" binding:"required"` // 文件名
	FileSize int64  `json:"file_size" binding:"required"` // 文件大小（字节）
}

// COSCredentials 腾讯云COS临时凭证
type COSCredentials struct {
	TmpSecretID  string `json:"tmpSecretId"`
	TmpSecretKey string `json:"tmpSecretKey"`
	SessionToken string `json:"sessionToken"`
}

// COSTokenResponse 腾讯云COS临时token响应
type COSTokenResponse struct {
	Credentials  *COSCredentials `json:"credentials"`
	RequestID    string          `json:"requestId"`
	Expiration   string          `json:"expiration"`
	StartTime    string          `json:"startTime"`
	ExpiredTime  int64           `json:"expiredTime"`
	Bucket       string          `json:"bucket"`
	Region       string          `json:"region"`
	AllowPrefix  string          `json:"allowPrefix"`
	AllowActions []string        `json:"allowActions"`
	UploadURL    string          `json:"uploadUrl"`
	Key          string          `json:"key"` // 生成的文件路径
}

// COSUploadResult COS上传结果
type COSUploadResult struct {
	Key      string `json:"key"`
	Location string `json:"location"`
	Bucket   string `json:"bucket"`
	ETag     string `json:"etag"`
}

// COSTokenResp 统一响应格式
type COSTokenResp struct {
	Result *COSTokenResponse `json:"result"`
}
