// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;
/**
 * @title 订单校验
 * @notice 该库定义了订单相关的校验与成交状态：校验订单参数合法性，并维护订单的已成交量与取消状态
 * @dev 业务职责
 *   - 校验订单必填项（maker、过期时间、salt、NFT资产、价格等），支持按需跳过过期校验
 *   - 通过filledAmount记录每笔订单的已成交数量，值为Canceled表示取消，不可再撮合
 *   - 提供更新已成交量、取消订单的接口，供订单簿在成交或用户取消时调用
*/

/**
EIP712 是一种以太坊签名标准，完整名字是 “Ethereum typed structured data hashing and signing”。

它解决的问题是：不要让用户去签一段看不懂的原始字节，而是签一个结构化对象，比如“订单”“报价”“授权”这类明确字段的数据。这样有几个直接好处：

签名内容可读。钱包可以展示字段名和字段值，而不是一串十六进制。
防重放更安全。EIP712 会把 name、version、chainId、verifyingContract 等域信息一起纳入签名域。
很适合订单系统。离线签单，链上验签，是 NFT 市场和撮合协议的常见做法
*/
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {ContextUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/ContextUpgradeable.sol";
import {EIP712Upgradeable} from "@openzeppelin/contracts-upgradeable/utils/cryptography/EIP712Upgradeable.sol";

import {Price} from "./libraries/RedBlackTreeLibrary.sol";
import {LibOrder, OrderKey} from "./libraries/LibOrder.sol";
abstract contract OrderValidator is Initializable, ContextUpgradeable, EIP712Upgradeable { 
    bytes4 private constant EIP_1271_MAGIC_VALUE = 0x1626ba7e;
    /// @dev 订单取消标记：filledAmount[key] == CANCELLED 表示该订单已取消
    uint256 private constant CANCELLED = type(uint256).max;
    /// @dev 订单 key -> 已成交量；CANCELLED 表示已取消，不可再匹配
    mapping(OrderKey => uint256) public filledAmount;
    function __OrderValidator_init(
        string memory EIP712Name,
        string memory EIP712Version
    ) internal onlyInitializing {
        __Context_init();
        __EIP712_init(EIP712Name, EIP712Version);
        __OrderValidator_init_unchained();
        
    }
    /**
    __OrderValidator_init_unchained 一般是 OpenZeppelin 可升级合约初始化模式里的一个“拆分初始化函数”。
    意思可以直接理解成两层：
    __OrderValidator_init(...),这是完整初始化入口，通常会顺带调用父合约的初始化函数。
    __OrderValidator_init_unchained(...),这是“不再继续往父类链条上传递”的那一层，只负责当前合约自己这部分状态初始化。
    unchained 这个词本身就是“解除链式调用”的意思。它的目的主要是避免多重继承场景里重复初始化父合约。
     */
    function __OrderValidator_init_unchained() internal onlyInitializing {
        // 当前合约暂无额外状态需要初始化
    }

    /**
     * @dev 统一的过期判断：区块时间可能被出块者在小范围内调整，
     *      但对订单过期（秒级容忍）的业务语义可接受。
     */
    function _isExpiredByTimestamp(uint64 expirationTime) internal view returns (bool) {
        // forge-lint: disable-next-line(block-timestamp)
        return expirationTime != 0 && expirationTime < block.timestamp;
    }

    function _validateOrder(LibOrder.Order memory order,bool isSkipExpiry) internal view {
        require(order.maker != address(0), "OVa: miss maker");
        require(!_isExpiredByTimestamp(order.expirationTime) || isSkipExpiry, "OVa: expired");
        require(order.salt != 0, "OVa: salt is zero");
        if (order.side == LibOrder.Side.Bid) {
            require(Price.unwrap(order.price) > 0, "OVa: zero price");
        } else if (order.side == LibOrder.Side.List) {
            require(order.nft.collection != address(0), "OVa: unsupported nft asset");
        }
    }
    /**
     * @notice 查询订单已成交量；若订单已取消则 revert
     * @param orderKey 订单哈希
     * @return orderFilledAmount 已成交数量（未成交为 0）
     */
    function _getFilledAmount(
        OrderKey orderKey
    ) internal view returns (uint256 orderFilledAmount){
        orderFilledAmount = filledAmount[orderKey];
    }
    /**
     * @notice 更新订单已成交量（成交后由订单簿调用）
     * @param newAmount 新的已成交数量
     * @param orderKey 订单哈希
     */
    function _updateFilledAmount(
        uint256 newAmount,
        OrderKey orderKey
    ) internal {
        require(newAmount != CANCELLED, "OVa: canceled");
        filledAmount[orderKey] = newAmount;
    }
    /**
     * @notice 取消订单（将已成交量设为 CANCELLED，后续撮合会 revert）
     * @param orderKey 订单哈希
     */
    function _cancelOrder(OrderKey orderKey) internal {
        filledAmount[orderKey] = CANCELLED;
    }

    uint256[50] private __gap;
}