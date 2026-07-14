// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;

import {Price} from "./RedBlackTreeLibrary.sol";
type OrderKey is bytes32;
library LibOrder { 
    enum Side {
        List, // 卖出
        Bid   // 买出
    }
    enum SaleKind {
        FixedPriceForCollection, // 市价
        FixedPriceForItem   // 限价
    }
    /**
     * @dev NFT 资产结构体
     */
    struct Asset{
        uint256 tokenId;
        address collection;
        uint amount;
    }
    /**
     * @dev NFT 信息结构体
     */
    struct NFTInfo{
        uint256 tokenId;
        address collection;
    }
    /**
     * @dev 订单结构体
     */
    struct Order{
        Side side; // 买/卖
        SaleKind saleKind; // 限价/市价
        address maker; // 订单发布者
        Asset nft; // 资产
        Price price; // 价格
        uint64 expirationTime; // 过期时间
        uint64 salt; // 随机数
    }
    /**
     * @dev 订单结构体
     */
    struct DBOrder {
        Order order;
        OrderKey next;
    }
    /**
     * @dev 订单队列
     */
    struct OrderQueue {
        OrderKey head;
        OrderKey tail;
    }
    struct EditDetail {
        OrderKey oldOrderKey; // old order key which need to be edit
        LibOrder.Order newOrder; // new order struct which need to be add
    }
    struct MatchDetail {
        LibOrder.Order sellOrder;
        LibOrder.Order buyOrder;
    }
    OrderKey public constant ORDERKEY_SENTINEL = OrderKey.wrap(0x0);
    bytes32 public constant ASSET_TYPEHASH =
        keccak256("Asset(uint256 tokenId,address collection,uint96 amount)");
    bytes32 public constant ORDER_TYPEHASH =
        keccak256(
            "Order(uint8 side,uint8 saleKind,address maker,Asset nft,uint128 price,uint64 expiry,uint64 salt)Asset(uint256 tokenId,address collection,uint96 amount)"
        );
    function hash(Asset memory asset) internal pure returns (bytes32) {
        return
            keccak256(
                abi.encode(
                    ASSET_TYPEHASH,
                    asset.tokenId,
                    asset.collection,
                    asset.amount
                )
            );
    }
    function hash(Order memory order) internal pure returns (OrderKey) {
        return
            OrderKey.wrap(
                keccak256(
                    abi.encodePacked(
                        ORDER_TYPEHASH,
                        order.side,
                        order.saleKind,
                        order.maker,
                        hash(order.nft),
                        Price.unwrap(order.price),
                        order.expirationTime,
                        order.salt
                    )
                )
            );
    }
    function isSentinel(OrderKey orderKey) internal pure returns (bool) {
        return OrderKey.unwrap(orderKey) == OrderKey.unwrap(ORDERKEY_SENTINEL);
    }

    function isNotSentinel(OrderKey orderKey) internal pure returns (bool) {
        return OrderKey.unwrap(orderKey) != OrderKey.unwrap(ORDERKEY_SENTINEL);
    }
}