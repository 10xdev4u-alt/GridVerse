const hre = require("hardhat");

async function main() {
  const [deployer] = await hre.ethers.getSigners();
  console.log("Deploying contracts with account:", deployer.address);

  const balance = await hre.ethers.provider.getBalance(deployer.address);
  console.log("Account balance:", hre.ethers.formatEther(balance));

  // 1. Deploy EnergyToken (constructor takes marketplace address; use deployer as placeholder)
  const EnergyToken = await hre.ethers.getContractFactory("EnergyToken");
  const energyToken = await EnergyToken.deploy(deployer.address);
  await energyToken.waitForDeployment();
  console.log("EnergyToken deployed to:", energyToken.target);

  // 2. Deploy EnergyMarketplace (linked to token)
  const EnergyMarketplace = await hre.ethers.getContractFactory("EnergyMarketplace");
  const marketplace = await EnergyMarketplace.deploy(energyToken.target);
  await marketplace.waitForDeployment();
  console.log("EnergyMarketplace deployed to:", marketplace.target);

  // 3. Grant MINTER_ROLE to marketplace on the token
  const MINTER_ROLE = await energyToken.MINTER_ROLE();
  const grantTx = await energyToken.grantRole(MINTER_ROLE, marketplace.target);
  await grantTx.wait();
  console.log("Granted MINTER_ROLE to EnergyMarketplace");

  // 4. Deploy Settlement (linked to token)
  const Settlement = await hre.ethers.getContractFactory("Settlement");
  const settlement = await Settlement.deploy(energyToken.target);
  await settlement.waitForDeployment();
  console.log("Settlement deployed to:", settlement.target);

  console.log("\n--- Deployment Complete ---");
  console.log("EnergyToken:", energyToken.target);
  console.log("EnergyMarketplace:", marketplace.target);
  console.log("Settlement:", settlement.target);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});