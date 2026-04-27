// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface IEnergyMarketplace {
    function purchaseEnergy(uint256 lotId) external payable;
    function listEnergy(uint256 amountKwh, uint256 pricePerKwh) external;
}

/**
 * @title ReentrancyAttacker
 * @notice Lists a lot as this contract. When someone buys it, the ETH transfer
 *         triggers receive(), which tries to call purchaseEnergy again via a
 *         low-level call (so a revert won't kill the outer transaction). The
 *         nonReentrant modifier on purchaseEnergy should cause the reentrant
 *         call to revert.
 */
contract ReentrancyAttacker {
    IEnergyMarketplace public marketplace;
    uint256 private _targetLot;

    /// @dev Track whether the reentrant attempt was blocked.
    bool public reentrancyBlocked;

    constructor(address _marketplace) {
        marketplace = IEnergyMarketplace(_marketplace);
    }

    /// @notice List attacker's own lot and remember which other lot to target for reentrancy.
    function setup(uint256 reenterLotId) external payable {
        _targetLot = reenterLotId;
        marketplace.listEnergy(1, msg.value);
    }

    /// @notice Called when someone buys our lot. Attempt reentrancy via low-level call.
    receive() external payable {
        if (_targetLot != 0) {
            uint256 lot = _targetLot;
            _targetLot = 0;

            // Use low-level call so the revert from nonReentrant does not
            // propagate and kill the entire payment flow.
            (bool ok, ) = address(marketplace).call(
                abi.encodeWithSignature("purchaseEnergy(uint256)", lot)
            );
            reentrancyBlocked = !ok;
        }
    }
}