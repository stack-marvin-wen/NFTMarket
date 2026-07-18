package types

// MetaNodeMintRequest MetaNodeNFT铸造请求参数
type MetaNodeMintRequest struct {
	ChainID     int    `json:"chain_id" binding:"required"`   // 链ID
	ToAddress   string `json:"to_address" binding:"required"` // 接收地址
	TokenURI    string `json:"token_uri" binding:"required"`  // NFT元数据URI
	Name        string `json:"name"`                          // NFT名称（可选，用于记录）
	Description string `json:"description"`                   // NFT描述（可选，用于记录）
}

// MetaNodeMintResult MetaNodeNFT铸造结果
type MetaNodeMintResult struct {
	TxHash      string `json:"tx_hash"`      // 交易哈希
	TokenID     string `json:"token_id"`     // 生成的Token ID
	Status      string `json:"status"`       // 交易状态: pending, confirmed, failed
	BlockNumber int64  `json:"block_number"` // 区块号（确认后）
	GasUsed     int64  `json:"gas_used"`     // 消耗的Gas
	GasPrice    string `json:"gas_price"`    // Gas价格
}

// MetaNodeMintResponse MetaNodeNFT铸造响应
type MetaNodeMintResponse struct {
	Result        *MetaNodeMintResult `json:"result"`
	TransactionID string              `json:"transaction_id"` // 交易ID（同tx_hash）
	TokenID       string              `json:"token_id"`       // Token ID
	ContractAddr  string              `json:"contract_addr"`  // 合约地址
	Message       string              `json:"message"`        // 响应消息
}

// MetaNodeContractInfo MetaNodeNFT合约信息
type MetaNodeContractInfo struct {
	Address     string `json:"address"`      // 合约地址
	Name        string `json:"name"`         // 合约名称 "MetaNodeNFT"
	Symbol      string `json:"symbol"`       // 合约符号 "MetaNode"
	Owner       string `json:"owner"`        // 合约所有者地址
	TotalSupply int64  `json:"total_supply"` // 总供应量
	ChainID     int    `json:"chain_id"`     // 链ID
}

// MetaNodeTokenInfo MetaNodeNFT Token信息
type MetaNodeTokenInfo struct {
	TokenID     string `json:"token_id"`     // Token ID
	Owner       string `json:"owner"`        // 当前所有者
	TokenURI    string `json:"token_uri"`    // 元数据URI
	Name        string `json:"name"`         // NFT名称
	Description string `json:"description"`  // NFT描述
	Image       string `json:"image"`        // 图片URL
	Attributes  string `json:"attributes"`   // 属性（JSON字符串）
	MintedAt    int64  `json:"minted_at"`    // 铸造时间戳
	MintTxHash  string `json:"mint_tx_hash"` // 铸造交易哈希
}

// MetaNodeBatchMintRequest 批量铸造请求
type MetaNodeBatchMintRequest struct {
	ChainID int                      `json:"chain_id" binding:"required"` // 链ID
	Mints   []MetaNodeSingleMintInfo `json:"mints" binding:"required"`    // 批量铸造信息
}

// MetaNodeSingleMintInfo 单个铸造信息
type MetaNodeSingleMintInfo struct {
	ToAddress   string `json:"to_address" binding:"required"` // 接收地址
	TokenURI    string `json:"token_uri" binding:"required"`  // NFT元数据URI
	Name        string `json:"name"`                          // NFT名称（可选）
	Description string `json:"description"`                   // NFT描述（可选）
}

// MetaNodeBatchMintResult 批量铸造结果
type MetaNodeBatchMintResult struct {
	TxHash       string   `json:"tx_hash"`       // 批量交易哈希
	TokenIDs     []string `json:"token_ids"`     // 生成的Token ID列表
	SuccessCount int      `json:"success_count"` // 成功数量
	FailedCount  int      `json:"failed_count"`  // 失败数量
	Status       string   `json:"status"`        // 整体状态
	GasUsed      int64    `json:"gas_used"`      // 消耗的Gas
}

// MetaNodeQueryRequest MetaNodeNFT查询请求
type MetaNodeQueryRequest struct {
	ChainID      int    `json:"chain_id"`      // 链ID
	ContractAddr string `json:"contract_addr"` // 合约地址
	TokenID      string `json:"token_id"`      // Token ID（可选）
	Owner        string `json:"owner"`         // 所有者地址（可选）
	Page         int    `json:"page"`          // 页码
	PageSize     int    `json:"page_size"`     // 每页大小
}

// MetaNodeQueryResponse MetaNodeNFT查询响应
type MetaNodeQueryResponse struct {
	Tokens []MetaNodeTokenInfo `json:"tokens"` // Token列表
	Total  int64               `json:"total"`  // 总数量
	Page   int                 `json:"page"`   // 当前页
	Size   int                 `json:"size"`   // 每页大小
}
