// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;

import {OrderKey, Price, LibOrder} from "../libraries/LibOrder.sol";
interface IEasySwapOrderBook { 
    /**
     * @dev 创建多个订单和交易对应资产
     * @param newOrders Array of orders to find matching orders for.
     * @return newOrderKeys of order keys for orders that can be filled by the given orders.
     */
    function makeOrders(
        LibOrder.Order[] calldata newOrders
    ) external payable returns (OrderKey[] memory newOrderKeys);
    /**
     * @dev 取消多个订单
     * @param orderKeys Array of orders to cancel.
     * @return successes of order keys for orders that were cancelled.
     */
    function cancelOrders(
        OrderKey[] calldata orderKeys
    ) external returns (bool[] memory successes);
    /**
     * @dev 修改多个订单
     * @param editDetails Array of orders to modify.
     * @return newOrderKeys of order keys for orders that were modified.
     */
    function editOrders(
        LibOrder.EditDetail[] calldata editDetails
    ) external payable returns (OrderKey[] memory newOrderKeys);
    function matchOrder(
        LibOrder.Order calldata sellOrder,
        LibOrder.Order calldata buyOrder
    ) external payable;
    /**
     * @dev Matches multiple orders atomically.
     * @dev If buying NFT, use the "valid sellOrder order" and construct a matching buyOrder order for order matching:
     * @dev    buyOrder.side = Bid, buyOrder.saleKind = FixedPriceForItem, buyOrder.maker = msg.sender,
     * @dev    nft and price values are the same as sellOrder, buyOrder.expiry > block.timestamp, buyOrder.salt != 0;
     * @dev If selling NFT, use the "valid buyOrder order" and construct a matching sellOrder order for order matching:
     * @dev    sellOrder.side = List, sellOrder.saleKind = FixedPriceForItem, sellOrder.maker = msg.sender,
     * @dev    nft and price values are the same as buyOrder, sellOrder.expiry > block.timestamp, sellOrder.salt != 0;
     * @param matchDetails Array of `MatchDetail` structs containing the details of sell and buy order to be matched.
     * @return successes Array of boolean values indicating the success of each match.
     */
    function matchOrders(
        LibOrder.MatchDetail[] calldata matchDetails
    ) external payable returns (bool[] memory successes);
    /**
     * @notice 聚合调用多个订单簿入口函数。
     * @dev 仅允许聚合 make/cancel/edit/match 相关函数，避免通过 multicall 调管理函数。
     * @dev 在一次 multicall 中，最多只能包含 1 个“可能消耗 msg.value”的子调用（makeOrders/editOrders/matchOrder/matchOrders）。
     * @param data 编码后的函数调用数据数组。
     * @param revertOnFail 为 true 时任一子调用失败将整笔回滚；为 false 时仅跳过失败子调用并继续。
     * @return successes 每个子调用是否成功。
     * @return results 每个子调用返回数据（失败时为 revert data）。
     */
    function multicall(
        bytes[] calldata data,
        bool revertOnFail
    ) external payable returns (bool[] memory successes, bytes[] memory results);
} 