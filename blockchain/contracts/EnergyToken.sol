// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";

/**
 * @title EnergyToken
 * @notice ERC-20 token representing energy credits where 1 token = 1 kWh.
 *         Only the minter role (assigned to the marketplace contract) can mint.
 *         Token holders can burn their own tokens.
 */
contract EnergyToken is ERC20, AccessControl {
    bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");

    constructor(address marketplace) ERC20("EnergyToken", "ENRG") {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(MINTER_ROLE, marketplace);
    }

    /**
     * @notice Mint tokens to a producer when surplus energy is reported.
     * @param to Recipient address
     * @param amount Amount in tokens (1:1 with kWh)
     */
    function mint(address to, uint256 amount) external onlyRole(MINTER_ROLE) {
        _mint(to, amount);
    }

    /**
     * @notice Burn tokens from caller's balance.
     * @param amount Amount to burn
     */
    function burn(uint256 amount) external {
        _burn(msg.sender, amount);
    }
}