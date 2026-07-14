// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;
/**
 * @title 订单库
 * @notice 按 collection / 买卖方向 / 价格 组织订单，支持按价格优先与时间优先查询与增删。
 * @dev 订单库的存储结构为：
 *      - 订单簿：使用红黑树按照价格排序（Bid侧高价优先，List侧低价优先），便于获取最优价和下一档价
 *      - 同一collection + side + price 的订单存储为链表订单队列
 *      - 提供add/remove订单、按照条件分页获取最优客成交订单等能力，供订单薄撮合和展示使用
 */

import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {RedBlackTreeLibrary, Price} from "./libraries/RedBlackTreeLibrary.sol";
import {LibOrder, OrderKey} from "./libraries/LibOrder.sol";
error CannotInsertDuplicateOrder(OrderKey orderKey);

contract OrderStorage is Initializable{
    using RedBlackTreeLibrary for RedBlackTreeLibrary.Tree;
    /// @dev 订单 key -> 订单数据（含订单体与链表 next），所有 key 使用 sentinel 包装避免碰撞
    mapping(OrderKey => LibOrder.DBOrder) internal orders; 
    /// @dev 每个 collection、买卖方向下的价格红黑树，用于按价格排序取最优价/下一档价
    mapping(address => mapping(LibOrder.Side => RedBlackTreeLibrary.Tree)) public priceTrees;
    /// @dev 每个 collection、方向、价格下的订单队列（链表头尾），同价按 orderKey 顺序（时间优先）
    mapping(address => mapping(LibOrder.Side => mapping(Price => LibOrder.OrderQueue))) public orderQueues;
    function __OrderStorage_init() internal onlyInitializing {}
    function __OrderStorage_init_unchained() internal onlyInitializing {}
    function onePlus(uint256 x) internal pure returns (uint256) {
        unchecked {
            return 1 + x;
        }
    }

    /**
     * @dev 统一过期判断：允许区块时间存在小范围偏移，
     *      对订单簿的过期过滤语义可接受。
     */
    function _isExpiredInOrderBook(uint64 expirationTime) internal view returns (bool) {
        // forge-lint: disable-next-line(block-timestamp)
        return expirationTime != 0 && expirationTime < block.timestamp;
    }

    /// @notice 获取某 collection、某方向下的最优价格（Bid 取最高价，List 取最低价）
    function getBestPrice(
        address collection,
        LibOrder.Side side
    ) public view returns (Price price) {
        price = (side == LibOrder.Side.Bid)
            ? priceTrees[collection][side].last()
            : priceTrees[collection][side].first();
    }

    /// @notice 获取当前价格档的下一档最优价（用于遍历价格档）
    function getNextBestPrice(
        address collection,
        LibOrder.Side side,
        Price price
    ) public view returns (Price nextBestPrice) {
        if (RedBlackTreeLibrary.isEmpty(price)) {
            nextBestPrice = (side == LibOrder.Side.Bid)
                ? priceTrees[collection][side].last()
                : priceTrees[collection][side].first();
        } else {
            nextBestPrice = (side == LibOrder.Side.Bid)
                ? priceTrees[collection][side].prev(price)
                : priceTrees[collection][side].next(price);
        }
    }
    /// @notice 将订单写入存储：若该价格档未在红黑树中则插入，并将订单加入对应价格下的队列尾部（同价时间优先）
    function _addOrder(LibOrder.Order memory order) internal returns (OrderKey orderKey){
        orderKey = LibOrder.hash(order);
        if (orders[orderKey].order.maker != address(0)) {
            revert CannotInsertDuplicateOrder(orderKey);
        }
        // 若该价格档尚未存在，插入价格树
        RedBlackTreeLibrary.Tree storage priceTree = priceTrees[order.nft.collection][order.side];
        if (!priceTree.exists(order.price)) {
            priceTree.insert(order.price);
        }
        LibOrder.OrderQueue storage orderQueue=orderQueues[order.nft.collection][order.side][order.price];
        // 将订单插入该 collection/side/price 下的订单队列
        if(LibOrder.isSentinel(orderQueue.head)){ // 队列为空，插入头尾
            orderQueues[order.nft.collection][order.side][order.price]= LibOrder.OrderQueue({head: orderKey, tail: orderKey});
            orderQueue = orderQueues[order.nft.collection][order.side][order.price];
        }
        if (LibOrder.isSentinel(orderQueue.tail)) { // 队尾端为空，插入头尾
            orderQueue.head = orderKey;
            orderQueue.tail = orderKey;
            orders[orderKey] = LibOrder.DBOrder(
                order,
                LibOrder.ORDERKEY_SENTINEL
            );
        }else {
            orders[orderQueue.tail].next = orderKey;
            orders[orderKey] = LibOrder.DBOrder(
                order,
                LibOrder.ORDERKEY_SENTINEL
            );
            orderQueue.tail = orderKey;
        }
    }
    function _removeOrder(LibOrder.Order memory order) internal returns(OrderKey orderKey) {
        LibOrder.OrderQueue storage orderQueue = orderQueues[order.nft.collection][order.side][order.price];
        orderKey = orderQueue.head; 
        OrderKey prevOrderKey;
        bool found = false;
        while(LibOrder.isNotSentinel(orderKey)&&!found){
            LibOrder.DBOrder memory dbOrder = orders[orderKey];
            if(
                (dbOrder.order.maker == order.maker) &&
                (dbOrder.order.saleKind == order.saleKind) &&
                (dbOrder.order.expirationTime == order.expirationTime) &&
                (dbOrder.order.salt == order.salt) &&
                (dbOrder.order.nft.tokenId == order.nft.tokenId) &&
                (dbOrder.order.nft.amount == order.nft.amount)
            ){
                OrderKey temp=orderKey;
                if(OrderKey.unwrap(orderQueue.head)==OrderKey.unwrap(orderKey)){
                    orderQueue.head=dbOrder.next;
                }else{
                    orders[prevOrderKey].next=dbOrder.next;
                }if (OrderKey.unwrap(orderQueue.tail) == OrderKey.unwrap(orderKey)) {
                    orderQueue.tail = prevOrderKey;
                }
                prevOrderKey = orderKey;
                orderKey = dbOrder.next;
                delete orders[temp];
                found = true;
            }else{
                prevOrderKey = orderKey;
                orderKey = dbOrder.next;
            }
        }
        if(found){
            if (LibOrder.isSentinel(orderQueue.head)){
                delete orderQueues[order.nft.collection][order.side][order.price];
                RedBlackTreeLibrary.Tree storage priceTree = priceTrees[order.nft.collection][order.side];
                if (priceTree.exists(order.price)) {
                    priceTree.remove(order.price);
                }
            }
        }else {
            revert("Cannot remove missing order");
        }
    }

    /**
     * @notice 按 collection/tokenId/方向/售卖类型分页查询订单（用于盘口展示、撮合遍历等）
     * @param collection NFT 合集地址
     * @param tokenId NFT tokenId（Bid 按单品时用于过滤）
     * @param side 买卖方向（Bid/List）
     * @param saleKind 售卖类型（一口价合集/单品等）
     * @param count 本页最多返回条数
     * @param price 当前价格档（可为空，为空则从最优价开始）
     * @param firstOrderKey 从哪一笔订单之后开始取（分页游标，可为 sentinel 表示从该价格档头开始）
     * @return resultOrders 本页订单列表
     * @return nextOrderKey 下一页起始 orderKey（用于下次请求）
     */
    function getOrders(
        address collection,
        uint256 tokenId,
        LibOrder.Side side,
        LibOrder.SaleKind saleKind,
        uint256 count,
        Price price,
        OrderKey firstOrderKey
    )
        external
        view
        returns (LibOrder.Order[] memory resultOrders, OrderKey nextOrderKey){
            resultOrders=new LibOrder.Order[](count);
            if(RedBlackTreeLibrary.isEmpty(price)){
                price=getBestPrice(collection,side);
            }else{
                if(LibOrder.isSentinel(firstOrderKey)){
                    price=getNextBestPrice(collection,side,price);
                }
            }

            uint256 i;
            while(RedBlackTreeLibrary.isEmpty(price)&&i<count){
                LibOrder.OrderQueue memory orderQueue=orderQueues[collection][side][price];
                OrderKey orderKey=orderQueue.head;
                if(LibOrder.isNotSentinel(firstOrderKey)){
                    while(LibOrder.isNotSentinel(orderKey)&&OrderKey.unwrap(orderKey)!=OrderKey.unwrap(firstOrderKey)){
                        LibOrder.DBOrder memory order=orders[orderKey];
                        orderKey=order.next;
                    }
                    firstOrderKey=LibOrder.ORDERKEY_SENTINEL;
                }
                while (LibOrder.isNotSentinel(orderKey) && i < count) {
                    LibOrder.DBOrder memory dbOrder = orders[orderKey];
                    orderKey = dbOrder.next;
                    if (_isExpiredInOrderBook(dbOrder.order.expirationTime)) continue;
                    if(side==dbOrder.order.side&&saleKind==LibOrder.SaleKind.FixedPriceForCollection){
                        if(dbOrder.order.side==LibOrder.Side.Bid && dbOrder.order.saleKind==LibOrder.SaleKind.FixedPriceForItem) continue;
                    }
                    if(side==LibOrder.Side.Bid&&saleKind==LibOrder.SaleKind.FixedPriceForItem){
                        if(dbOrder.order.side==LibOrder.Side.Bid && dbOrder.order.saleKind==LibOrder.SaleKind.FixedPriceForItem && tokenId != dbOrder.order.nft.tokenId) continue;
                    }
                    resultOrders[i]=dbOrder.order;
                    nextOrderKey=dbOrder.next;
                    i=onePlus(i);
                }
                price=getNextBestPrice(collection,side,price);
            }
    }
    /// @notice 获取某 collection、方向、售卖类型下当前最优可成交订单（价格优先，同价时间优先；会跳过过期订单与 tokenId 不匹配的 Bid）
    function getBestOrder(
        address collection,
        uint256 tokenId,
        LibOrder.Side side,
        LibOrder.SaleKind saleKind
    ) external view returns (LibOrder.Order memory orderResult){
        Price price=getBestPrice(collection,side);
        while(RedBlackTreeLibrary.isNotEmpty(price)){
            LibOrder.OrderQueue memory orderQueue=orderQueues[collection][side][price];
            OrderKey orderKey=orderQueue.head;
            while(LibOrder.isNotSentinel(orderKey)){
                LibOrder.DBOrder memory dbOrder=orders[orderKey];
                orderKey=dbOrder.next;
                if (_isExpiredInOrderBook(dbOrder.order.expirationTime)) continue;
                if(side==LibOrder.Side.Bid&&saleKind==LibOrder.SaleKind.FixedPriceForCollection){
                    if(dbOrder.order.side==LibOrder.Side.Bid && dbOrder.order.saleKind==LibOrder.SaleKind.FixedPriceForItem) continue;
                }
                if(side==LibOrder.Side.Bid&&saleKind==LibOrder.SaleKind.FixedPriceForItem){
                    if(dbOrder.order.side==LibOrder.Side.Bid && dbOrder.order.saleKind==LibOrder.SaleKind.FixedPriceForItem && tokenId != dbOrder.order.nft.tokenId) continue;
                }
                orderResult=dbOrder.order;
                return orderResult;
            }
            price=getNextBestPrice(collection,side,price);
        }
    }
    uint256[50] private __gap;
}