// SPDX-License-Identifier: MIT
pragma solidity ^0.8.35;
/**
 * @title 协议费/手续费管理
 * @notice 该库定义了协议费/手续费的收取与分配规则
 * @dev 业务职责
 *   - 存储protocolShare（通常为万分比或类似单位，具体由 LibPayInfo 约定上限）。
 *   - 仅 Owner 可修改 protocolShare，且不得超过 LibPayInfo.MAX_PROTOCOL_SHARE。
 *   - 被订单簿等合约继承，在分配款项时读取 protocolShare 计算并转给协议方。
 */
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {OwnableUpgradeable} from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";

import {LibPayInfo} from "./libraries/LibPayInfo.sol";

abstract contract ProtocolManager is
    Initializable,
    OwnableUpgradeable
{
    /// @dev 协议抽成比例（与 LibPayInfo 中单位一致，如万分比）
    uint128 public protocolShare;
    event LogUpdatedProtocolShare(uint128 indexed newProtocolShare);
    function __ProtocolManager_init(
        uint128 newProtocolShare
    ) internal onlyInitializing {
        __ProtocolManager_init_unchained(
            newProtocolShare
        );
    }

    function __ProtocolManager_init_unchained(
        uint128 newProtocolShare
    ) internal onlyInitializing {
        _setProtocolShare(newProtocolShare);
    }
    /// @notice 设置协议抽成比例（仅管理员），不得超过最大允许值
    function setProtocolShare(
        uint128 newProtocolShare
    ) external onlyOwner {
        _setProtocolShare(newProtocolShare);
    }
    function _setProtocolShare(uint128 newProtocolShare) internal {
        require(
            newProtocolShare <= LibPayInfo.MAX_PROTOCOL_SHARE,
            "PM: exceed max protocol share"
        );
        protocolShare = newProtocolShare;
        emit LogUpdatedProtocolShare(newProtocolShare);
    }
    uint256[50] private __gap;
}