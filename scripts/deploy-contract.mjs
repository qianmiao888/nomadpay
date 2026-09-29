import fs from 'node:fs';
import { ContractFactory, Interface, JsonRpcProvider, Wallet } from 'ethers';

const rpc = process.env.MONAD_RPC_URL;
const key = process.env.DEPLOYER_PRIVATE_KEY;
if (!rpc || !key) throw new Error('Set MONAD_RPC_URL and DEPLOYER_PRIVATE_KEY');
const artifact = JSON.parse(fs.readFileSync('artifacts/NomadPay.json', 'utf8'));
const wallet = new Wallet(key, new JsonRpcProvider(rpc));
const contract = await new ContractFactory(artifact.abi, artifact.bytecode, wallet).deploy();
const receipt = await contract.deploymentTransaction().wait();
console.log(`CONTRACT_ADDRESS=${await contract.getAddress()}`);
console.log(`START_BLOCK=${receipt.blockNumber}`);
console.log(`EVENT_TOPIC=${new Interface(artifact.abi).getEvent('InvoicePaid').topicHash}`);
