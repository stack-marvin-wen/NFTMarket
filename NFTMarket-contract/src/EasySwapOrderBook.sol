// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {ContextUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/ContextUpgradeable.sol";
import {OwnableUpgradeable} from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {PausableUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";

import {LibTransferSafeUpgradeable, IERC721} from "./libraries/LibTransferSafeUpgradeable.sol";
import {Price} from "./libraries/RedBlackTreeLibrary.sol";
import {LibOrder, OrderKey} from "./libraries/LibOrder.sol";
import {LibPayInfo} from "./libraries/LibPayInfo.sol";

import {IEasySwapOrderBook} from "./interfaces/IEasySwapOrderBook.sol";
import {IEasySwapVault} from "./interfaces/IEasySwapVault.sol";

import {OrderStorage} from "./OrderStorage.sol";
import {OrderValidator} from "./OrderValidator.sol";
import {ProtocolManager} from "./ProtocolManager.sol";
contract EasySwapOrderBook is
    IEasySwapOrderBook,
    Initializable,
    ContextUpgradeable,
    OwnableUpgradeable,
    ReentrancyGuard,
    PausableUpgradeable,
    OrderStorage,
    ProtocolManager,
    OrderValidator  {
    using LibTransferSafeUpgradeable for address;
    using LibTransferSafeUpgradeable for IERC721;

    /**
    * @dev 订单创建事件
    */
    event LogMake(
        OrderKey orderKey,
        LibOrder.Side indexed side,
        LibOrder.SaleKind indexed saleKind,
        address indexed maker,
        LibOrder.Asset nft,
        Price price,
        uint64 expirationTime,
        uint64 salt
    );
    /**
    * @dev 订单取消事件
    */
    event LogCancel(OrderKey indexed orderKey,address indexed maker);
    /**
    * @dev 订单匹配事件
    */
    event LogMatch(
        OrderKey indexed makerOrderKey,
        OrderKey indexed takerOrderKey,
        LibOrder.Order sellOrder,
        LibOrder.Order buyOrder,
        uint128 fillPrice
    );
    /**
    * @dev ETH提取事件
    */
    event LogWithdrawETH(address recipient, uint256 amount);
    /**
    * @dev 批量匹配错误事件
    */
    event MulticallInnerError(uint256 offset, bytes msg);
    /**
    * @dev 批量匹配内部错误事件
    */
    event BatchMatchInnerError(uint256 offset,bytes msg);
    /**
    * @dev 跳过订单事件
    */
    event LogSkipOrder(OrderKey orderKey,uint64 salt);

    /**
    * @dev 仅允许代理调用
    */
    modifier onlyDelegateCall() {
        _checkDelegateCall();
        _;
    }
    /**
    * @notice 合约自身地址，用于检查是否为delegatecall
    */
    address private immutable self = address(this);
    /**
    * @notice 资产存储合约地址
    */
    address private _vault;
    /**
    * @notice 初始化合约
    * @param newVault 资产存储合约地址
    * @param EIP712Name EIP712 名称
    * @param EIP712Version EIP712 版本
    * @param newProtocolShare 协议抽成比例
    */
    function initialize(uint128 newProtocolShare,address newVault,string memory EIP712Name,string memory EIP712Version) external initializer {
        __EasySwapOrderBook_init(newProtocolShare,newVault,EIP712Name,EIP712Version);
    }

    function __EasySwapOrderBook_init(uint128 newProtocolShare,address newVault,string memory EIP712Name,string memory EIP712Version) internal onlyInitializing {
        __EasySwapOrderBook_init_unchained(
            newProtocolShare,
            newVault,
            EIP712Name,
            EIP712Version
        );
    }
    function __EasySwapOrderBook_init_unchained(uint128 newProtocolShare,address newVault,string memory EIP712Name,string memory EIP712Version) internal onlyInitializing{
        __Context_init();
        __Ownable_init(_msgSender());
        __Pausable_init();

        __OrderStorage_init();
        __ProtocolManager_init(newProtocolShare);
        __OrderValidator_init(EIP712Name, EIP712Version);

        setVault(newVault);
    }

        


    /**
    * @notice 批量创建订单并转移相关资产
    * @dev 业务逻辑说明：
    *      1. List订单（挂单）：需要先授权EasySwapVault合约，创建订单时会将NFT转移到订单池（金库）
    *      2. Bid订单（出价）：需要传入ETH作为出价金额，创建订单时会将ETH转移到订单池（金库）
    * 
    * @dev 订单验证规则：
    *      - order.maker 必须是 msg.sender（只能为自己创建订单）
    *      - order.price 不能为 0
    *      - order.expiration 必须大于当前区块时间戳，或为 0（表示永不过期）
    *      - order.salt 不能为 0
    * 
    * @param newOrders 多个订单结构数据数组
    * @return newOrderKeys 返回订单唯一标识数组，如果某个订单创建失败，对应位置返回空标识
    */
    function makeOrders(
        LibOrder.Order[] calldata newOrders
    ) external payable override whenNotPaused nonReentrant returns (OrderKey[] memory newOrderKeys){
        uint256 orderAmount = newOrders.length;
        newOrderKeys = new OrderKey[](orderAmount);
        uint256 ETHAmount; // 累计需要的ETH总金额（仅Bid订单需要）
        for(uint256 i=0;i<orderAmount;i++){
            uint128 buyPrice; // Bid订单的出价金额
            if(newOrders[i].side == LibOrder.Side.Bid){
                // 计算Bid订单需要的ETH：单价 × 数量
                buyPrice = Price.unwrap(newOrders[i].price) * uint128(newOrders[i].nft.amount);
            }

            OrderKey newOrderKey = _makeOrderTry(newOrders[i],buyPrice);
            newOrderKeys[i] = newOrderKey;
            if(OrderKey.unwrap(newOrderKey) != OrderKey.unwrap(LibOrder.ORDERKEY_SENTINEL)){
                ETHAmount += buyPrice;
            }
        }
        if(msg.value > ETHAmount){
            // 如果传入的ETH多于实际需要的金额，退回多余的ETH
            // 如果ETH不足，交易会回滚
            _msgSender().safeTransferETH(msg.value - ETHAmount);
        }
    }
    /**
        * @notice 批量取消订单
        * @dev 业务逻辑说明：
        *     - 只有订单创建者可以取消自己的订单
        *     - 已完全成交的订单无法取消
        *     - 取消List订单：从金库提取NFT返回给创建者
        *     - 取消Bid订单：从金库提取未成交ETH返回给创建者
        */
    function cancelOrders(OrderKey[] calldata orderKeys) external override whenNotPaused nonReentrant returns (bool[] memory successes){
        successes=new bool[](orderKeys.length);
        for(uint256 i=0;i<orderKeys.length;i++){
            successes[i]=_cancelOrderTry(orderKeys[i]);
        }
    }
    /**
        * @notice 批量修改订单
        * @param editDetails 修改订单详情
        * @dev 业务逻辑说明：
        *      - newOrder的saleKind、side、maker、nft必须与oldOrderKey对应的订单匹配，否则会被跳过
        *      - 只能修改价格（price）和数量（amount）
        *      - newOrder的expiry和salt可以重新生成
        * @dev 资产处理说明：
        *      - List订单：直接更新金库中的订单关联
        *      - Bid订单：如果新价格更高，需要补足差额ETH；如果新价格更低，会退回多余ETH
        * @param editDetails 批量修改订单详情
        */
    function editOrders(LibOrder.EditDetail[] calldata editDetails) external override payable whenNotPaused nonReentrant returns (OrderKey[] memory newOrderKeys){
        newOrderKeys=new OrderKey[](editDetails.length);
        uint256 bidETHAmount;
        for(uint256 i=0;i<editDetails.length;i++){
                (OrderKey newOrderKey, uint256 bidPrice) = _editOrderTry(editDetails[i].oldOrderKey,editDetails[i].newOrder);
                bidETHAmount += bidPrice;
                newOrderKeys[i] = newOrderKey;
        }
        if (msg.value > bidETHAmount) {
        // 如果传入的ETH多于实际需要的金额，退回多余的ETH
            _msgSender().safeTransferETH(msg.value - bidETHAmount);
        }
    }
    /**
        * @notice 匹配单个订单
        */
    function matchOrder(LibOrder.Order calldata sellOrder,LibOrder.Order calldata buyOrder) external override payable whenNotPaused nonReentrant{
        uint256 costValue = _matchOrder(sellOrder, buyOrder, msg.value);
        if (msg.value > costValue) {
            // 如果传入的ETH多于实际需要的金额，退回多余的ETH
            _msgSender().safeTransferETH(msg.value - costValue);
        }
    }
    /**
        * @notice 批量匹配订单
        */
    function matchOrders(LibOrder.MatchDetail[] calldata matchDetails) external payable override whenNotPaused nonReentrant returns (bool[] memory successes){
        successes = new bool[](matchDetails.length);
        uint128 buyETHAmount; // 累计需要的ETH总金额（仅Bid订单需要）
        for(uint256 i=0;i<matchDetails.length;i++){
            LibOrder.MatchDetail calldata matchDetail = matchDetails[i];
            // 使用delegatecall调用内部匹配函数，保持相同的存储上下文
            (bool success, bytes memory data) = address(this).delegatecall(
                abi.encodeWithSignature(
                    "matchOrderWithoutPayback((uint8,uint8,address,(uint256,address,uint96),uint128,uint64,uint64),(uint8,uint8,address,(uint256,address,uint96),uint128,uint64,uint64),uint256)",
                    matchDetail.sellOrder,
                    matchDetail.buyOrder,
                    msg.value - buyETHAmount
                )
            );
            if (success) {
                successes[i] = success;
                if (matchDetail.buyOrder.maker == _msgSender()) {
                    // 如果是买家发起的匹配，累计已花费的ETH
                    uint128 buyPrice;
                    buyPrice = abi.decode(data, (uint128));
                    buyETHAmount += buyPrice;
                }
            } else {
                // 记录批量匹配中的错误
                emit BatchMatchInnerError(i, data);
            }
        }
        if (msg.value > buyETHAmount) {
            // 如果传入的ETH多于实际需要的金额，退回多余的ETH
            _msgSender().safeTransferETH(msg.value - buyETHAmount);
        }
    }
    /**
    * @notice 聚合执行多个订单簿调用（单笔交易内串联多个操作）
    * @dev 仅支持交易相关入口函数：
    *      - makeOrders / cancelOrders / editOrders / matchOrder / matchOrders
    * @dev 通过 delegatecall 保持 msg.sender 为用户本人，从而兼容 maker 权限校验。
    * @dev 由于 delegatecall 下每个子调用看到的 msg.value 相同，为避免资金语义歧义，
    *      一次 multicall 最多允许 1 个“可能消耗 msg.value”的子调用。
    * @param data ABI 编码后的调用数组
    * @param revertOnFail 为 true 时任一失败将整笔回滚；为 false 时仅记录失败并继续
    * @return successes 每个子调用是否成功
    * @return results 每个子调用返回数据（失败则为 revert data）
    */
    function multicall(bytes[] calldata data,bool revertOnFail) external override payable whenNotPaused nonReentrant returns (bool[] memory successes, bytes[] memory results){
        uint256 callAmount = data.length;
        successes = new bool[](callAmount);
        results = new bytes[](callAmount);
        uint256 valueSensitiveCallAmount;
        for (uint256 i = 0; i < callAmount; ++i){
            bytes calldata callData = data[i];
            bytes4 selector;
            if (callData.length >= 4) {
                assembly {
                    selector := calldataload(callData.offset)
                }
            }
            require(_isSupportedMulticallSelector(selector), "HD: unsupported selector");
            if (_isValueSensitiveSelector(selector)) {
                ++valueSensitiveCallAmount;
                require(valueSensitiveCallAmount <= 1, "HD: multi value calls");
            }
            (bool success, bytes memory result) = address(this).delegatecall(callData);
            successes[i] = success;
            results[i] = result;

            if (!success) {
                emit MulticallInnerError(i, result);
                if (revertOnFail) {
                    assembly {
                        revert(add(result, 32), mload(result))
                    }
                }
            }
        }
    }



    function setVault(address newVault) public onlyOwner {
        require(newVault != address(0), "HD: zero address");
        _vault = newVault;
    }

    /**
    * @notice 匹配订单但不退回多余ETH（内部函数，仅用于批量匹配）
    * @dev 此函数只能通过delegatecall调用，用于批量匹配时避免多次退回ETH
    * @param sellOrder 挂单订单
    * @param buyOrder 出价订单
    * @param msgValue 传入的ETH金额
    * @return costValue 实际花费的ETH金额
    */
    function matchOrderWithoutPayback(
        LibOrder.Order calldata sellOrder,
        LibOrder.Order calldata buyOrder,
        uint256 msgValue
    )
        external
        payable
        whenNotPaused
        onlyDelegateCall
        returns (uint128 costValue)
    {
        costValue = _matchOrder(sellOrder, buyOrder, msgValue);
    }
    function _makeOrderTry(LibOrder.Order calldata order,uint128 ETHAmount) internal returns (OrderKey newOrderKey){
        if(order.maker == _msgSender() && Price.unwrap(order.price) !=0 && order.salt != 0 && !_isExpiredByTimestamp(order.expirationTime) && filledAmount[LibOrder.hash(order)] == 0){
            newOrderKey = LibOrder.hash(order);

            if(order.side == LibOrder.Side.List){
                // List订单限制数量为1
                if (order.nft.amount != 1) return LibOrder.ORDERKEY_SENTINEL;
                IEasySwapVault(_vault).depositNFT(newOrderKey, order.maker, order.nft.collection, order.nft.tokenId);
            }else if(order.side == LibOrder.Side.Bid){
                if (order.nft.amount == 0) return LibOrder.ORDERKEY_SENTINEL;
                IEasySwapVault(_vault).depositETH{value: uint256(ETHAmount)}(newOrderKey, ETHAmount);
            }
            // 将订单添加到订单存储
            _addOrder(order);
            // 发出订单创建事件
            emit LogMake(
                newOrderKey,
                order.side,
                order.saleKind,
                order.maker,
                order.nft,
                order.price,
                order.expirationTime,
                order.salt
            );
        }else{
            // 订单创建失败，发出跳过事件
            emit LogSkipOrder(LibOrder.hash(order), order.salt);
        }
    }

    function _cancelOrderTry(OrderKey orderKey) internal returns (bool success){
        LibOrder.Order memory order = orders[orderKey].order;
        if(order.maker == _msgSender() && filledAmount[orderKey] < order.nft.amount){ // 订单创建者才能取消订单，并且订单没有成交
            OrderKey orderHash=LibOrder.hash(order);
            _removeOrder(order);
            if(order.side == LibOrder.Side.List){
                IEasySwapVault(_vault).withdrawNFT(orderHash, order.maker, order.nft.collection, order.nft.tokenId);
            }else if(order.side == LibOrder.Side.Bid){
                uint256 availNFTamount = order.nft.amount - filledAmount[orderKey]; // 可退的ETH数量
                IEasySwapVault(_vault).withdrawETH(orderHash, Price.unwrap(order.price) * availNFTamount, order.maker);
            }
            _cancelOrder(orderKey);
            success = true;
            emit LogCancel(orderKey, order.maker);
        }else{
            success = false;
            // 取消失败，发出跳过事件
            emit LogSkipOrder(orderKey, order.salt);
        }
    }

    function _editOrderTry(OrderKey oldOrderKey, LibOrder.Order memory newOrder) internal returns (OrderKey newOrderKey, uint256 bidPrice){
        LibOrder.Order memory oldOrder = orders[oldOrderKey].order;

        // 只能编辑价格和数量
        if(oldOrder.side != newOrder.side || 
            oldOrder.saleKind != newOrder.saleKind || 
            oldOrder.maker != newOrder.maker || 
            oldOrder.nft.collection != newOrder.nft.collection || 
            oldOrder.nft.tokenId != newOrder.nft.tokenId ||
            filledAmount[oldOrderKey] >= oldOrder.nft.amount){
            emit LogSkipOrder(oldOrderKey, oldOrder.salt);
            return (LibOrder.ORDERKEY_SENTINEL, 0);
        }
        if (newOrder.maker != _msgSender() || newOrder.salt == 0 || _isExpiredByTimestamp(newOrder.expirationTime) || filledAmount[LibOrder.hash(newOrder)] != 0) {
            emit LogSkipOrder(oldOrderKey, oldOrder.salt);
            return (LibOrder.ORDERKEY_SENTINEL, 0);
        }
        // 取消旧订单
        uint256 oldFilledAmount = filledAmount[oldOrderKey];
        _removeOrder(oldOrder);
        _cancelOrder(oldOrderKey);
        emit LogCancel(oldOrderKey, oldOrder.maker);
        // 创建新订单
        newOrderKey = _addOrder(newOrder);

        if (oldOrder.side == LibOrder.Side.List) {
            IEasySwapVault(_vault).editNFT(oldOrderKey, newOrderKey);
        }else if(oldOrder.side == LibOrder.Side.Bid) {
            uint256 oldTotalPrice = Price.unwrap(oldOrder.price) * (oldOrder.nft.amount-oldFilledAmount); // 旧订单剩余价格
            uint256 newTotalPrice = Price.unwrap(newOrder.price) * newOrder.nft.amount; // 新订单总价格
            if (newTotalPrice > oldTotalPrice) { // 补差价
                bidPrice = newTotalPrice - oldTotalPrice;
                IEasySwapVault(_vault).editETH{value: uint256(bidPrice)}(oldOrderKey, newOrderKey, oldTotalPrice, newTotalPrice,oldOrder.maker);
            } else  {
                IEasySwapVault(_vault).editETH(oldOrderKey, newOrderKey, oldTotalPrice, newTotalPrice,oldOrder.maker);
            }
        }
        // 发出新订单创建事件
        emit LogMake(
            newOrderKey,
            newOrder.side,
            newOrder.saleKind,
            newOrder.maker,
            newOrder.nft,
            newOrder.price,
            newOrder.expirationTime,
            newOrder.salt
        );
    }

    function _matchOrder(LibOrder.Order calldata sellOrder,LibOrder.Order calldata buyOrder,uint256 msgValue) internal returns (uint128 costValue){
        OrderKey sellOrderKey=LibOrder.hash(sellOrder);
        OrderKey buyOrderKey=LibOrder.hash(buyOrder);
        _isMatchAvailable(sellOrder,buyOrder,sellOrderKey,buyOrderKey);
        if(_msgSender()==sellOrder.maker){
            // 场景1：卖家接受出价（卖家主动匹配买家的Bid订单）
            require(msgValue==0, "HD: value > 0"); // 卖家接受出价时不需要传入ETH
            bool isSellExist=orders[sellOrderKey].order.maker!=address(0); // 检查sellOrder是否存在于订单存储中
            _validateOrder(sellOrder,isSellExist);
            _validateOrder(orders[buyOrderKey].order,false); // 检查buyOrder（Bid订单必须存在于订单存储中）
            uint128 fillPrice=Price.unwrap(buyOrder.price); // 成交价格为买家出价
            if(isSellExist){ 
                // 如果sellOrder存在于订单存储中，一处并标记为完全成交
                _removeOrder(sellOrder);
                _updateFilledAmount(sellOrder.nft.amount,sellOrderKey); // 标记为完全成交
            }
            // 更新buyOrder的已成交数量
            _updateFilledAmount(filledAmount[buyOrderKey]+1,buyOrderKey); // 标记为完全成交
            emit LogMatch(
                sellOrderKey,
                buyOrderKey,
                sellOrder,
                buyOrder,
                fillPrice
            );
            // 资产转移
            IEasySwapVault(_vault).withdrawETH(buyOrderKey, fillPrice, address(this)); // 将买家出价的ETH转给卖家
            // 计算手续费(扣除手续费转给卖家)
            uint256 protocolFee = _shareToAmount(fillPrice, protocolShare);
            sellOrder.maker.safeTransferETH(fillPrice - protocolFee);
            // 转移NFT给买家
            if (isSellExist) {
                IEasySwapVault(_vault).withdrawNFT(sellOrderKey, buyOrder.maker, sellOrder.nft.collection, sellOrder.nft.tokenId);
            }else{
                // 如果sellOrder不存在于订单存储中，说明卖家没有将NFT托管到金库中，需要直接从卖家转给买家
                IEasySwapVault(_vault).transferERC721(sellOrder.maker, buyOrder.maker, sellOrder.nft);
            }
        }else if (_msgSender() == buyOrder.maker){
            // 场景2：买家接受挂单（买家主动匹配卖家的List订单）
            bool isBuyExist=orders[buyOrderKey].order.maker!=address(0); // 检查buyOrder是否存在于订单存储中
            _validateOrder(orders[sellOrderKey].order,false); // 检查sellOrder（List订单必须存在于订单存储中）
            _validateOrder(buyOrder,isBuyExist);
            uint128 fillPrice=Price.unwrap(sellOrder.price); // 成交价格
            uint128 buyPrice=Price.unwrap(buyOrder.price); // 买家出价
            if(!isBuyExist){
                // 如果buyOrder不存在于订单存储中，需要传入足够的ETH
                require(msgValue >= fillPrice, "HD: value < fill price");
            }else{
                require(buyPrice >= fillPrice, "HD: buy price < fill price");
                IEasySwapVault(_vault).withdrawETH(buyOrderKey, buyPrice, address(this)); // 将买家出价的ETH转给卖家
                _removeOrder(buyOrder);
                _updateFilledAmount(filledAmount[buyOrderKey] + 1,buyOrderKey); // 标记为完全成交
            }
            _updateFilledAmount(sellOrder.nft.amount,sellOrderKey); // 标记为完全成交
            emit LogMatch(
                buyOrderKey,
                sellOrderKey,
                buyOrder,
                sellOrder,
                fillPrice
            );
            // 计算协议手续费并转给卖家（扣除手续费后的金额）
            uint128 protocolFee = _shareToAmount(fillPrice, protocolShare);
            sellOrder.maker.safeTransferETH(fillPrice - protocolFee);
            // 如果买家出价高于成交价，退回多余ETH
            if (buyPrice > fillPrice) {
                buyOrder.maker.safeTransferETH(buyPrice - fillPrice);
            }
            // 从金库提取NFT给买家
            IEasySwapVault(_vault).withdrawNFT(sellOrderKey, buyOrder.maker, sellOrder.nft.collection, sellOrder.nft.tokenId);
            // 返回实际花费的ETH金额（如果buyOrder已存在于订单存储中，则不需要额外ETH）
            costValue = isBuyExist ? 0 : buyPrice;
        }else {
            revert("HD: sender invalid");
        }
    }

    function _isMatchAvailable(LibOrder.Order calldata sellOrder,LibOrder.Order calldata buyOrder,OrderKey sellOrderKey,OrderKey buyOrderKey) internal view{
        require(OrderKey.unwrap(sellOrderKey) != OrderKey.unwrap(buyOrderKey), "HD: same order");
        require(sellOrder.side == LibOrder.Side.List && buyOrder.side == LibOrder.Side.Bid, "HD: side mismatch");
        require(sellOrder.saleKind == LibOrder.SaleKind.FixedPriceForItem, "HD: kind mismatch");
        require(sellOrder.maker != buyOrder.maker, "HD: same maker");
        require(buyOrder.saleKind == LibOrder.SaleKind.FixedPriceForCollection || (sellOrder.nft.collection == buyOrder.nft.collection && sellOrder.nft.tokenId == buyOrder.nft.tokenId), "HD: asset mismatch");
        require(filledAmount[sellOrderKey] < sellOrder.nft.amount && filledAmount[buyOrderKey] < buyOrder.nft.amount, "HD: order closed");
    } 
    function _shareToAmount(
        uint128 total,
        uint128 share
    ) internal pure returns (uint128) {
        return (total * share) / LibPayInfo.TOTAL_SHARE;
    }   
    function _checkDelegateCall() private view {
        require(address(this) != self);
    }
    /**
    * @notice 检查 selector 是否允许通过 multicall 聚合
    */
    function _isSupportedMulticallSelector(
        bytes4 selector
    ) private pure returns (bool) {
        return
            selector == this.makeOrders.selector ||
            selector == this.cancelOrders.selector ||
            selector == this.editOrders.selector ||
            selector == this.matchOrder.selector ||
            selector == this.matchOrders.selector;
    }
    /**
    * @notice 标记可能消费 msg.value 的 selector
    */
    function _isValueSensitiveSelector(
        bytes4 selector
    ) private pure returns (bool) {
        return
            selector == this.makeOrders.selector ||
            selector == this.editOrders.selector ||
            selector == this.matchOrder.selector ||
            selector == this.matchOrders.selector;
    }
    /**
    * @notice 提取ETH（仅所有者，用于提取协议手续费）
    * @param recipient 接收地址
    * @param amount 提取金额
    */
    function withdrawETH(
        address recipient,
        uint256 amount
    ) external nonReentrant onlyOwner {
        recipient.safeTransferETH(amount);
        emit LogWithdrawETH(recipient, amount);
    }
    /**
     * @notice 暂停合约（仅所有者）
     * @dev 暂停后所有交易功能将无法使用
     */
    function pause() external onlyOwner {
        _pause();
    }

    /**
     * @notice 恢复合约（仅所有者）
     * @dev 恢复后所有交易功能将重新可用
     */
    function unpause() external onlyOwner {
        _unpause();
    }

    /**
     * @notice 接收ETH的回退函数
     * @dev 用于接收Bid订单的ETH和协议手续费
     */
    receive() external payable {}

    /**
     * @notice 可升级合约的存储间隙
     * @dev 为未来升级预留存储空间
     */
    uint256[50] private __gap;
}